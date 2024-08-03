#include <SimpleFOC.h>
#include <Wire.h>


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
  RUNNING
};

enum ConnectionState
{
  WAITING_FOR_CONNECTION,
  CONNECTED
};

unsigned long last_keepalive_time = 0;
const unsigned long KEEPALIVE_TIMEOUT = 30 * 1000; // 60 seconds

unsigned long last_heartbeat_time = 0;
const unsigned long HEARTBEAT_PERIOD = 5 * 1000; // 10 seconds

DeviceState current_device_state = UNINITIALIZED;
ConnectionState current_connection_state = WAITING_FOR_CONNECTION;

struct CommandEntry
{
  const char *command;
  void (*function)();
  DeviceState required_state;
};

void keepalive();
void ack_alive();
void ack_handshake();
void handle_init();
void handle_reset();

const CommandEntry command_table[] = {
    {"init", handle_init, UNINITIALIZED},
    {"keepalive", ack_alive, RUNNING},
    {"handshake", ack_handshake, RUNNING},
    {"reset", handle_reset, RUNNING}
};
const int command_table_size = sizeof(command_table) / sizeof(CommandEntry);

BLDCMotor motor = BLDCMotor(POLE_PAIRS);
BLDCDriver6PWM driver = BLDCDriver6PWM(A_PHASE_UH, A_PHASE_UL, A_PHASE_VH, A_PHASE_VL, A_PHASE_WH, A_PHASE_WL);
LowsideCurrentSense current_sense = LowsideCurrentSense(SHUNT_RESISTOR, OPAMP_GAIN, A_OP1_OUT, A_OP2_OUT, A_OP3_OUT);
MagneticSensorI2C sensor = MagneticSensorI2C(AS5600_I2C);
Commander command = Commander(Serial);


void do_motor(char *cmd)
{
  char cmd_copy[32];
  strncpy(cmd_copy, cmd, sizeof(cmd_copy) - 1);
  cmd_copy[sizeof(cmd_copy) - 1] = '\0';

  command.motor(&motor, cmd);

  SimpleFOCDebug::print("ACK_MOTOR: ");
  SimpleFOCDebug::println(cmd_copy);
}

void print_device_serial_no()
{
  char serial[25];
  uint32_t *uniqueId = (uint32_t *)0x1FFF7590;
  sprintf(serial, "%08lX%08lX%08lX", uniqueId[2], uniqueId[1], uniqueId[0]);
  SimpleFOCDebug::print("SERIAL_NO ");
  SimpleFOCDebug::println(serial);
}

void keepalive(){
  last_keepalive_time = millis();
}

void heartbeat(){
  last_heartbeat_time = millis();
  Serial.println("HEARTBEAT");
}

void ack_alive(){
  keepalive();
  Serial.println("ACK_KEEPALIVE");
}

void ack_handshake(){
  Serial.println("ACK_HANDSHAKE");
  current_connection_state = CONNECTED;
  keepalive();
}

void reset_board()
{
  Serial.println("ACK_RESET");
  delay(1000);
  NVIC_SystemReset();
}

void wait_for_handshake() {
  static String buffer = "";
  unsigned long startTime = millis();
  while (millis() - startTime < 5000) { // 5-second timeout
    while (Serial.available() > 0) {
      char inChar = (char)Serial.read();
      buffer += inChar;
      if (buffer.indexOf("HANDSHAKE") != -1) {
        Serial.println("ACK_HANDSHAKE");
        current_connection_state = CONNECTED;
        keepalive();
        buffer = "";  // Clear the buffer
        return;
      }
      // Trim the buffer if it gets too long without finding a match
      if (buffer.length() > 20) {
        buffer = buffer.substring(buffer.length() - 10);
      }
    }
    delay(10); // Small delay to prevent tight looping
  }
  Serial.println("WAITING_FOR_CONNECTION");
}

void setup()
{
  Serial.begin(BAUD_RATE);
  current_device_state = UNINITIALIZED;
  current_connection_state = WAITING_FOR_CONNECTION;
  while (!Serial) {
    ; // wait for serial port to connect. Needed for native USB port only
  }
  Serial.println("WAITING_FOR_CONNECTION");
}

void on_demand_setup()
{
  Wire.setClock(WIRE_FREQ);
  SimpleFOCDebug::enable();

  motor.useMonitoring(Serial);

  driver.voltage_power_supply = 12;
  driver.voltage_limit = 10;
  motor.current_limit = 1;

  driver.pwm_frequency = DRIVER_PWM_FREQ;
  sensor.init();
  driver.init();
  motor.linkSensor(&sensor);
  motor.linkDriver(&driver);

  current_sense.linkDriver(&driver);

  motor.PID_current_q.P = 5;
  motor.PID_current_q.I = 1000;
  motor.PID_current_q.D = 0;
  motor.PID_current_q.limit = motor.voltage_limit;
  motor.PID_current_q.output_ramp = 1e6;
  motor.LPF_current_q.Tf = 0.005;

  motor.PID_velocity.P = 0.751;
  motor.PID_velocity.I = 2.672;
  motor.PID_velocity.D = 0.00005;

  motor.PID_velocity.output_ramp = 100000.0;
  motor.LPF_velocity.Tf = 0.05;
  motor.PID_velocity.limit = 50;

  motor.init();
  current_sense.init();
  motor.linkCurrentSense(&current_sense);

  motor.monitor_variables = 0;

  motor.initFOC();

  command.add('M', do_motor, (char *)"motor");

  Serial.print("INIT_FINISHED: ");
  print_device_serial_no();
}

void process_command(const String &cmd)
{
  for (const auto &entry : command_table)
  {
    if (cmd.equalsIgnoreCase(entry.command))
    {
      if (current_device_state == entry.required_state || entry.required_state == RUNNING)
      {
        entry.function();
      }
      else
      {
        Serial.println("COMMAND_NOT_ALLOWED");
      }
      return;
    }
  }
  Serial.print("UNKNOWN_COMMAND:");
  Serial.println(cmd);
  keepalive();
}

void handle_init()
{
  if (current_device_state == UNINITIALIZED)
  {
    Serial.println("ACK_INIT");
    current_device_state = INITIALIZING;
    on_demand_setup();
    Serial.println("ACK_RUNNING");
    current_device_state = RUNNING;
  }
  else
  {
    Serial.println("Board is already initialized");
  }
}

void handle_reset()
{
  reset_board();
}

void handle_uninitialized_state()
{
  if (digitalRead(A_BUTTON) == LOW)
  {
    Serial.println("BUTTON_INIT");
    handle_init();
  }
  else if (Serial.available())
  {
    String command = Serial.readStringUntil('\n');
    command.trim();
    process_command(command);
  }
}

void handle_active_state()
{
  motor.loopFOC();
  motor.move();
  motor.monitor();
  command.run();

  if (Serial.available())
  {
    String serial_command = Serial.readStringUntil('\n');
    process_command(serial_command);
  }
}

void loop()
{
  switch (current_connection_state)
  {
    case WAITING_FOR_CONNECTION:
      wait_for_handshake();
      return;

    case CONNECTED:
      if (millis() - last_keepalive_time > KEEPALIVE_TIMEOUT)
      {
        current_connection_state = WAITING_FOR_CONNECTION;
        Serial.println("WAITING_FOR_CONNECTION");
        return;
      }
      break;
  }

  switch (current_device_state)
  {
    case UNINITIALIZED:
      handle_uninitialized_state();
      break;

    case INITIALIZING:
      // This state is handled in on_demand_setup()
      break;

    case RUNNING:
      handle_active_state();
      break;
  }

  if (millis() - last_heartbeat_time > HEARTBEAT_PERIOD)
  {
    heartbeat();
  }
}