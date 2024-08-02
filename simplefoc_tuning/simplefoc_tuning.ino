#include <SimpleFOC.h>
#include <Wire.h>

#include "ButtonHandler.h"

#define BAUD_RATE 1000000
#define WIRE_FREQ 400000
#define POLE_PAIRS 7
#define SHUNT_RESISTOR 0.003f
#define OPAMP_GAIN 9.142857143f
#define DRIVER_PWM_FREQ 20000

enum DeviceState
{
  UNINITIALIZED,
  INITIALIZING,
  TESTING,
  WAITING_FOR_CONNECTION,  // New state
  CONNECTED,               // Rename WAITING_FOR_COMMAND to CONNECTED
  RUNNING
};

// Add these global variables
unsigned long last_keepalive_time = 0;
const unsigned long KEEPALIVE_TIMEOUT = 5000; // 5 seconds

// Add these global variables
DeviceState current_state = UNINITIALIZED;
unsigned long last_log_time = 0;
unsigned long keep_alive_time = 0;
// Define a structure for command table entries
struct CommandEntry
{
  const char *command;
  void (*function)();
  DeviceState required_state;
};

// Function prototypes for command handlers
void handle_init();
void handle_test();
void handle_reset();

// Command table
const CommandEntry command_table[] = {
    {"init", handle_init, UNINITIALIZED},
    {"test", handle_test, WAITING_FOR_COMMAND},
    {"reset", handle_reset, WAITING_FOR_COMMAND}};
const int command_table_size = sizeof(command_table) / sizeof(CommandEntry);

BLDCMotor motor = BLDCMotor(POLE_PAIRS);
BLDCDriver6PWM driver = BLDCDriver6PWM(A_PHASE_UH, A_PHASE_UL, A_PHASE_VH, A_PHASE_VL, A_PHASE_WH, A_PHASE_WL);
LowsideCurrentSense current_sense = LowsideCurrentSense(SHUNT_RESISTOR, OPAMP_GAIN, A_OP1_OUT, A_OP2_OUT, A_OP3_OUT);
MagneticSensorI2C sensor = MagneticSensorI2C(AS5600_I2C);
// include commander interface
Commander command = Commander(Serial);
const int RESET_PRESS_DURATION = 3000; // 3 seconds for long press
const int DOUBLE_PRESS_INTERVAL = 500; // 500ms window for double press

ButtonHandler button(A_BUTTON, DOUBLE_PRESS_INTERVAL, RESET_PRESS_DURATION); // pin, double click interval, long press interval

bool setup_done = false;
int button_state = 0;

enum TestState
{
  IDLE,
  VELOCITY_FORWARD,
  VELOCITY_BACKWARD,
  ANGLE_FORWARD,
  ANGLE_BACKWARD,
  COMPLETED
};

TestState testState = IDLE;
unsigned long stateStartTime = 0;
unsigned long currentTime = 0;
int angleStep = 0;
float startAngle = 0;

void run_test_routine()
{
  if (testState == IDLE)
  {
    Serial.println("Starting test routine...");
    motor.controller = MotionControlType::velocity;
    motor.torque_controller = TorqueControlType::voltage;
    motor.move(10);
    testState = VELOCITY_FORWARD;
    stateStartTime = millis();
    Serial.println("State: VELOCITY_FORWARD");
  }
}

void update_test_routine()
{
  currentTime = millis();
  switch (testState)
  {
  case VELOCITY_FORWARD:
    if (currentTime - stateStartTime >= 2000)
    {
      Serial.println("State: VELOCITY_BACKWARD");
      motor.move(-10);
      testState = VELOCITY_BACKWARD;
      stateStartTime = currentTime;
    }
    break;
  case VELOCITY_BACKWARD:
    if (currentTime - stateStartTime >= 2000)
    {
      Serial.println("Stopping velocity test");
      motor.move(0);
      delay(500); // Short delay to allow motor to stop
      Serial.println("State: ANGLE_FORWARD");
      motor.controller = MotionControlType::angle;
      startAngle = motor.shaft_angle;
      angleStep = 0;
      testState = ANGLE_FORWARD;
      stateStartTime = currentTime;
    }
    break;
  case ANGLE_FORWARD:
    if (currentTime - stateStartTime >= 1000)
    {
      angleStep++;
      if (angleStep <= 4)
      {
        float targetAngle = startAngle + _PI_2 * angleStep;
        Serial.print("Moving to angle step ");
        Serial.print(angleStep);
        Serial.print(": ");
        Serial.println(targetAngle);
        motor.move(targetAngle);
        stateStartTime = currentTime;
      }
      else
      {
        Serial.println("State: ANGLE_BACKWARD");
        angleStep = 4;
        testState = ANGLE_BACKWARD;
        stateStartTime = currentTime;
      }
    }
    break;
  case ANGLE_BACKWARD:
    if (currentTime - stateStartTime >= 1000)
    {
      angleStep--;
      if (angleStep >= 0)
      {
        float targetAngle = startAngle + _PI_2 * angleStep;
        Serial.print("Moving back to angle step ");
        Serial.print(angleStep);
        Serial.print(": ");
        Serial.println(targetAngle);
        motor.move(targetAngle);
        stateStartTime = currentTime;
      }
      else
      {
        testState = COMPLETED;
      }
    }
    break;
  case COMPLETED:
    Serial.println("Test routine completed. Switching to torque mode.");
    motor.controller = MotionControlType::velocity;
    motor.torque_controller = TorqueControlType::voltage;
    motor.move(0); // Set initial torque to 0
    testState = IDLE;
    break;
  default:
    break;
  }
}

void do_motor(char *cmd)
{
  // Create a copy of the command string
  char cmd_copy[32]; // Adjust the size if needed to accommodate longer commands
  strncpy(cmd_copy, cmd, sizeof(cmd_copy) - 1);
  cmd_copy[sizeof(cmd_copy) - 1] = '\0'; // Ensure null-termination

  // Process the command
  command.motor(&motor, cmd);

  // Print the processed command using the copy
  SimpleFOCDebug::print("ACK: Motor command processed: ");
  SimpleFOCDebug::println(cmd_copy);
}

void print_device_serial_no()
{
  char serial[25];
  uint32_t *uniqueId = (uint32_t *)0x1FFF7590;
  sprintf(serial, "%08lX%08lX%08lX", uniqueId[2], uniqueId[1], uniqueId[0]);
  SimpleFOCDebug::print("Device Serial Number: ");
  SimpleFOCDebug::println(serial);
}

void reset_board()
{
  Serial.println("Reset command received. Resetting board...");
  delay(1000);
  NVIC_SystemReset();
}

void perform_handshake()
{
  while (current_state == WAITING_FOR_CONNECTION)
  {
    if (Serial.available() && Serial.read() == 'H')
    {
      Serial.println("ACK_HANDSHAKE");
      current_state = CONNECTED;
      last_keepalive_time = millis();
    }
  }
}

void setup()
{
  Serial.begin(BAUD_RATE);
  current_state = WAITING_FOR_CONNECTION;
  Serial.println("WAITING_FOR_CONNECTION");
}

void on_demand_setup()
{
  Wire.setClock(WIRE_FREQ);
  SimpleFOCDebug::enable();

  motor.useMonitoring(Serial);

  // 20kHz is the standard for many boards,
  // but the B-G431B-ESC1 can do 30kHz.
  // could stay lower if we need to allow more time for more complex calculation.
  // as we are using current sensing: https://docs.simplefoc.com/low_side_current_sense
  // the higher, the smoother.

  driver.voltage_power_supply = 12;
  driver.voltage_limit = 10;
  motor.current_limit = 1; // Amps - default 0.2Amps

  driver.pwm_frequency = DRIVER_PWM_FREQ;
  sensor.init();
  driver.init();
  motor.linkSensor(&sensor);
  motor.linkDriver(&driver);

  current_sense.linkDriver(&driver);

  // PID parameters - default 
  motor.PID_current_q.P = 5;                       
  motor.PID_current_q.I = 1000;                   
  motor.PID_current_q.D = 0;
  motor.PID_current_q.limit = motor.voltage_limit; 
  motor.PID_current_q.ramp = 1e6;                  
  // Low pass filtering - default 
  LPF_current_q.Tf= 0.005;                 

  motor.PID_velocity.P = 0.751;
  motor.PID_velocity.I = 2.672;
  motor.PID_velocity.D = 0.00005;

  motor.PID_velocity.output_ramp = 100000.0;
  motor.LPF_velocity.Tf = 0.05;
  motor.PID_velocity.limit = 50; // 0 means no limit

  motor.init();
  current_sense.init();
  motor.linkCurrentSense(&current_sense);

  // motor.monitor_variables = 0xFF; // Monitor all values
  // motor.monitor_variables = _MON_TARGET | _MON_VEL | _MON_ANGLE;  // default _MON_TARGET | _MON_VOLT_Q | _MON_VEL | _MON_ANGLE
  motor.monitor_variables = 0;

  motor.initFOC();

  command.add('M', do_motor, (char *)"motor");
  // command.add('B', toggle_debug, (char*)"toggle debug mode");
  Serial.print("Finished board initalization for: ");
  print_device_serial_no();
  current_state = WAITING_FOR_COMMAND;
}

void process_command(const String &cmd)
{
  for (int i = 0; i < command_table_size; i++)
  {
    if (cmd.equalsIgnoreCase(command_table[i].command))
    {
      if (current_state == command_table[i].required_state || command_table[i].required_state == WAITING_FOR_COMMAND)
      {
        command_table[i].function();
      }
      else
      {
        Serial.println("Command not allowed in current state");
      }
      return;
    }
  }
  Serial.println("Unknown command");
}

// Add this function for periodic logging
void log_status()
{
  Serial.print("State: ");
  Serial.print(current_state);
  // Serial.print(", Test State: ");
  // Serial.print(testState);
  Serial.print(", Velocity: ");
  Serial.print(motor.shaft_velocity);
  Serial.print(", Angle: ");
  Serial.println(motor.shaft_angle);
}

void start_test_routine()
{
  if (current_state == WAITING_FOR_COMMAND)
  {
    Serial.println("Test command received. Starting test routine...");
    run_test_routine();
    current_state = TESTING;
  }
  else
  {
    Serial.println("Test cannot be started in current state");
  }
}

// Command handler functions
void handle_init()
{
  if (current_state == UNINITIALIZED)
  {
    Serial.println("Command triggered board init");
    Serial.println("ACK_INIT: INITIALIZING"); // Add this line
    current_state = INITIALIZING;
    on_demand_setup();
    Serial.println("ACK_INIT: WAITING_FOR_COMMAND"); // Add this line
    current_state = WAITING_FOR_COMMAND;
  }
  else
  {
    Serial.println("Board is already initialized");
  }
}

void handle_test()
{
  if (current_state == WAITING_FOR_COMMAND)
  {
    start_test_routine();
    current_state = TESTING;
  }
  else
  {
    Serial.println("Cannot start test in current state");
  }
}

void handle_reset()
{
  reset_board();
}

void loop()
{
  if (current_state == WAITING_FOR_CONNECTION)
  {
    perform_handshake();
    return;
  }

  // Check for keepalive timeout
  if (millis() - last_keepalive_time > KEEPALIVE_TIMEOUT)
  {
    current_state = WAITING_FOR_CONNECTION;
    Serial.println("DISCONNECTED_TIMEOUT");
    return;
  }

  switch (button.checkButton())
  {
  case SINGLE_PRESS:
    handle_init();
    break;
  case DOUBLE_PRESS:
    handle_test();
    break;
  case LONG_PRESS:
    handle_reset();
    break;
  default:
    break;
  }

  switch (current_state)
  {
  case UNINITIALIZED:
    if (digitalRead(A_BUTTON) == LOW)
    {
      Serial.println("Button triggered board init");
      handle_init();
    }
    else if (Serial.available())
    {
      String command = Serial.readStringUntil('\n');
      command.trim();
      process_command(command);
    }
    break;

  case INITIALIZING:
    // This state is handled in on_demand_setup()
    break;

  case TESTING:
    if (testState != IDLE)
    {
      update_test_routine();
    }
    if (testState == COMPLETED)
    {
      current_state = WAITING_FOR_COMMAND;
    }
    break;

  case CONNECTED:
  case RUNNING:
    motor.loopFOC();
    motor.move();
    motor.monitor();
    command.run();

    // Process serial commands
    if (Serial.available())
    {
      String serial_command = Serial.readStringUntil('\n');
      serial_command.trim();
      if (serial_command == "KEEPALIVE")
      {
        last_keepalive_time = millis();
        Serial.println("ACK_KEEPALIVE");
      }
      else
      {
        process_command(serial_command);
      }
    }
      break;
  }
}