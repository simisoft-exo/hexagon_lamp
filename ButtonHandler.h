// ButtonHandler.h

#ifndef BUTTON_HANDLER_H
#define BUTTON_HANDLER_H

#include <Arduino.h>

enum ButtonEvent {
  NONE,
  SINGLE_PRESS,
  DOUBLE_PRESS,
  LONG_PRESS
};

class ButtonHandler {
public:
  ButtonHandler(int buttonPin, unsigned long doubleInterval, unsigned long longInterval);
  ButtonEvent checkButton();

private:
  const int pin;
  const unsigned long doubleClickInterval;
  const unsigned long longPressInterval;
  
  unsigned long lastPressTime;
  unsigned long pressStartTime;
  bool wasPressed;
  int pressCount;
};

#endif