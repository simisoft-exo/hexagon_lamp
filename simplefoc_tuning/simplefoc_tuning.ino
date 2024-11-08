#include <SimpleFOC.h>
#include "SimpleFOCDrivers.h"
#include "utilities/stm32math/STM32G4CORDICTrigFunctions.h"


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

unsigned long last_heartbeat_time = 0;
const unsigned long HEARTBEAT_PERIOD = 15 * 1000; // 5 seconds


DeviceState current_device_state = UNINITIALIZED;

struct CommandEntry
{
  const char *command;
  void (*function)();
};

void heartbeat()
{
  if(current_device_state == RUNNING)
  {
    Serial.println("HB");
    last_heartbeat_time = millis();
  }
}


void ack_handshake();
void handle_init();
void handle_reset();
void print_device_serial_no();

void doHandshake(char* cmd) { ack_handshake(); }
void doInit(char* cmd) { handle_init(); }
void doReset(char* cmd) { handle_reset(); }
void doSerialNo(char* cmd) { print_device_serial_no(); }

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

  SimpleFOCDebug::print("K_MOT: ");
  Serial.println(cmd_copy);
}

String get_serial_number() {
  char serial[25];
  uint32_t *uniqueId = (uint32_t *)0x1FFF7590;
  sprintf(serial, "%08lX%08lX%08lX", uniqueId[2], uniqueId[1], uniqueId[0]);
  return String(serial);
}

void print_device_serial_no() {
  String serial = get_serial_number();
  Serial.println("SERIAL_NO:");
  Serial.println(serial);
}

void ack_handshake() {
  // String serial = get_serial_number();
  Serial.println("K");
  // Serial.println(serial);
}

void reset_board()
{
  Serial.println("K_RESET");
  delay(1000);
  NVIC_SystemReset();
}

void setup()
{
  Serial.begin(BAUD_RATE);
  current_device_state = UNINITIALIZED;
  pinMode(A_BUTTON, INPUT_PULLUP);
  while (!Serial) {
    ; // wait for serial port to connect. Needed for native USB port only
  }
  SimpleFOCDebug::enable(&Serial);
  command.add('H', doHandshake, "handshake");
  command.add('I', doInit, "init");
  command.add('R', doReset, "reset");
  command.add('S', doSerialNo, "serial_no");
  Serial.print("K_SETUP");
}


String interpretFOCStatus(FOCMotorStatus status) {
  switch(status) {
    case FOCMotorStatus::motor_uninitialized:
      return "M_UNINITIALIZED";
    case FOCMotorStatus::motor_initializing:
      return "M_INITIALIZING";
    case FOCMotorStatus::motor_uncalibrated:
      return "M_UNCALIBRATED";
    case FOCMotorStatus::motor_calibrating:
      return "M_CALIBRATING";
    case FOCMotorStatus::motor_ready:
      return "M_READY";
    case FOCMotorStatus::motor_error:
      return "M_ERROR";
    case FOCMotorStatus::motor_calib_failed:
      return "M_CALIB_FAILED";
    case FOCMotorStatus::motor_init_failed:
      return "M_INIT_FAILED";
    default:
      return "UNKNOWN_STATUS";
  }
}

bool on_demand_setup()
{
  Wire.setClock(WIRE_FREQ);
  SimpleFOC_CORDIC_Config();
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

  String status_str = interpretFOCStatus(motor.motor_status);

  if (motor.motor_status != FOCMotorStatus::motor_ready) {
    Serial.println("INIT_FAILED");
    return false;
  }

  command.add('M', do_motor, (char *)"motor");


  Serial.println(status_str);
  print_device_serial_no();
  return true;
}

void handle_init()
{
  if (current_device_state == UNINITIALIZED)
  {
    Serial.println("K_INIT");
    current_device_state = INITIALIZING;
    if (on_demand_setup())
    {
      Serial.println("K_RUNNING");
      current_device_state = RUNNING;
    }
    else
    {
      current_device_state = UNINITIALIZED;
    }
  }
  else
  {
    Serial.println("ALREADY_INITED");
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

  command.run();

}

void handle_active_state()
{
  motor.loopFOC();
  motor.move();
  motor.monitor();
  command.run();
}

void loop()
{
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