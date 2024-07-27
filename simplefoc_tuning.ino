#include <SimpleFOC.h>
#include <Wire.h>

#include "ButtonHandler.h"


#define BAUD_RATE 1000000
#define WIRE_FREQ 400000
#define POLE_PAIRS 7
#define SHUNT_RESISTOR 0.003f
#define OPAMP_GAIN 9.142857143f
#define DRIVER_PWM_FREQ 20000

BLDCMotor motor = BLDCMotor(POLE_PAIRS);
BLDCDriver6PWM driver = BLDCDriver6PWM(A_PHASE_UH, A_PHASE_UL, A_PHASE_VH, A_PHASE_VL, A_PHASE_WH, A_PHASE_WL);
LowsideCurrentSense current_sense = LowsideCurrentSense(SHUNT_RESISTOR, OPAMP_GAIN, A_OP1_OUT, A_OP2_OUT, A_OP3_OUT);
MagneticSensorI2C sensor = MagneticSensorI2C(AS5600_I2C);

// include commander interface
Commander command = Commander(Serial);
const int RESET_PRESS_DURATION = 3000; // 3 seconds for long press
const int DOUBLE_PRESS_INTERVAL = 500; // 500ms window for double press

ButtonHandler button(A_BUTTON, DOUBLE_PRESS_INTERVAL,RESET_PRESS_DURATION);  // pin, double click interval, long press interval

bool setup_done = false;
int button_state = 0;


enum TestState {
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

void run_test_routine() {
  if (testState == IDLE) {
    Serial.println("Starting test routine...");
    motor.controller = MotionControlType::velocity;
    motor.torque_controller = TorqueControlType::voltage;
    motor.move(10);
    testState = VELOCITY_FORWARD;
    stateStartTime = millis();
    Serial.println("State: VELOCITY_FORWARD");
  }
}

void update_test_routine() {
  currentTime = millis();
  switch (testState) {
    case VELOCITY_FORWARD:
      if (currentTime - stateStartTime >= 2000) {
        Serial.println("State: VELOCITY_BACKWARD");
        motor.move(-10);
        testState = VELOCITY_BACKWARD;
        stateStartTime = currentTime;
      }
      break;
    case VELOCITY_BACKWARD:
      if (currentTime - stateStartTime >= 2000) {
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
      if (currentTime - stateStartTime >= 1000) {
        angleStep++;
        if (angleStep <= 4) {
          float targetAngle = startAngle + _PI_2 * angleStep;
          Serial.print("Moving to angle step ");
          Serial.print(angleStep);
          Serial.print(": ");
          Serial.println(targetAngle);
          motor.move(targetAngle);
          stateStartTime = currentTime;
        } else {
          Serial.println("State: ANGLE_BACKWARD");
          angleStep = 4;
          testState = ANGLE_BACKWARD;
          stateStartTime = currentTime;
        }
      }
      break;
    case ANGLE_BACKWARD:
      if (currentTime - stateStartTime >= 1000) {
        angleStep--;
        if (angleStep >= 0) {
          float targetAngle = startAngle + _PI_2 * angleStep;
          Serial.print("Moving back to angle step ");
          Serial.print(angleStep);
          Serial.print(": ");
          Serial.println(targetAngle);
          motor.move(targetAngle);
          stateStartTime = currentTime;
        } else {
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


void do_motor(char* cmd) {
  command.motor(&motor, cmd);
  SimpleFOCDebug::print("ACK: Motor command processed: ");
  SimpleFOCDebug::println(cmd);
}

void print_device_serial_no() {
  char serial[25];
  uint32_t *uniqueId = (uint32_t *)0x1FFF7590;
  sprintf(serial, "%08lX%08lX%08lX", uniqueId[2], uniqueId[1], uniqueId[0]);
  SimpleFOCDebug::print("Device Serial Number: ");
  SimpleFOCDebug::println(serial);
}

void setup() {
  Serial.begin(BAUD_RATE);
  pinMode(A_BUTTON, INPUT);
  Serial.println("Press button to initialize...");
}

void on_demand_setup() {
  Wire.setClock(WIRE_FREQ);
  SimpleFOCDebug::enable();

  motor.useMonitoring(Serial);
 
  // 20kHz is the standard for many boards,
  // but the B-G431B-ESC1 can do 30kHz. 
  //could stay lower if we need to allow more time for more complex calculation.
  //as we are using current sensing: https://docs.simplefoc.com/low_side_current_sense
  //the higher, the smoother.

  driver.voltage_power_supply = 12;
  driver.voltage_limit = 10;
  motor.current_limit = 1; // Amps - default 0.2Amps


  driver.pwm_frequency = DRIVER_PWM_FREQ; 
  sensor.init();
  driver.init();
  motor.linkSensor(&sensor);
  motor.linkDriver(&driver);

  current_sense.linkDriver(&driver);
  

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

  command.add('M', do_motor, (char*)"motor");
  // command.add('B', toggle_debug, (char*)"toggle debug mode");
  Serial.print("Finished board initalization for: ");
  print_device_serial_no();

  _delay(300);
  setup_done = true;

}

void loop() {
  if (setup_done) {
    motor.loopFOC();
    motor.move();
    motor.monitor();
    command.run();

    // Update test routine if i2t's running
    if (testState != IDLE) {
      update_test_routine();
    }

    // Check for button events
    switch (button.checkButton()) {
      case SINGLE_PRESS:
      case DOUBLE_PRESS:
        if (testState == IDLE) {
          run_test_routine();
        } else {
          Serial.println("Test already in progress");
        }
        break;
      case LONG_PRESS:
        Serial.println("Long press detected. Resetting board...");
        delay(1000);
        NVIC_SystemReset();
        break;
      default:
        break;
    }

    // Add periodic logging
    static unsigned long lastLogTime = 0;
    if (currentTime - lastLogTime >= 500) { // Log every 500ms
      Serial.print("State: ");
      Serial.print(testState);
      Serial.print(", Velocity: ");
      Serial.print(motor.shaft_velocity);
      Serial.print(", Angle: ");
      Serial.println(motor.shaft_angle);
      lastLogTime = currentTime;
    }
  } else {
    if (digitalRead(A_BUTTON) == LOW || Serial.available()) {
      if (Serial.available()) {
        String command = Serial.readStringUntil('\n');
        command.trim();
        if (command.equalsIgnoreCase("init")) {
          Serial.println("Command triggered board init");
          on_demand_setup();
        } else {
          Serial.println("Unknown command. Use 'init' to initialize.");
        }
      } else {
        Serial.println("Button triggered board init");
        on_demand_setup();
      }
    }
  }
}