// ButtonHandler.cpp

#include "ButtonHandler.h"

ButtonHandler::ButtonHandler(int buttonPin, unsigned long doubleInterval, unsigned long longInterval)
  : pin(buttonPin), doubleClickInterval(doubleInterval), longPressInterval(longInterval),
    lastPressTime(0), pressStartTime(0), wasPressed(false), pressCount(0) {
  pinMode(pin, INPUT_PULLUP);
}

ButtonEvent ButtonHandler::checkButton() {
  int buttonState = digitalRead(pin);
  unsigned long currentTime = millis();
  ButtonEvent event = NONE;

  if (buttonState == LOW && !wasPressed) {
    wasPressed = true;
    pressStartTime = currentTime;
    pressCount++;

    if (currentTime - lastPressTime < doubleClickInterval) {
      event = DOUBLE_PRESS;
      pressCount = 0;
    }

    lastPressTime = currentTime;
  } 
  else if (buttonState == HIGH && wasPressed) {
    wasPressed = false;
    if (currentTime - pressStartTime >= longPressInterval) {
      event = LONG_PRESS;
    } else if (pressCount == 1) {
      event = SINGLE_PRESS;
    }
    pressCount = 0;
  }
  else if (buttonState == LOW && wasPressed) {
    if (currentTime - pressStartTime >= longPressInterval) {
      event = LONG_PRESS;
      wasPressed = false;
      pressCount = 0;
    }
  }

  return event;
}