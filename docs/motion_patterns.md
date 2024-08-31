# Motors Patterns Protocol

## Introduction

The Motors Patterns Protocol is a custom communication protocol designed for controlling and interacting with motors in a structured manner. It provides a set of commands and data structures to manage and manipulate motor patterns on devices equipped with the Hexagon platform.

## Protocol Overview

The protocol is based on a client-server model, where the device acts as the server and the mobile application acts as the client.


### MotorPattern

The `MotorPattern` data type represents a pattern for a motor. It contains the following fields:

- `motorId`: The identifier of the motor this pattern is for.
- `segments`: A list of motor motion segments, where each segment contains:
  - `velocity`: The speed and direction of the motor (-70 to 70, where negative values indicate reverse), in radians per second
  - `duration`: The time in milliseconds for which this segment should run.

#### Example:

```json
{"patterns": 
[{
  "motorId": 0,
  "segments": [
    {
      "velocity": 30,
      "duration": 1000,
    },
    {
      "velocity": 0,
      "duration": 500,
    }]}
  ]
}
```


}

### HexagonMotions

The `HexagonMotions` data type represents coordinated patterns for all motors in the hexagon. It contains the following fields:

- `name`: A string identifier for the motion set.
- `patterns`: An array of 7 `MotorPattern` objects, one for each motor (IDs 0 to 6).

#### Example:








### Scheduler

The scheduler is responsible for managing and executing a list of motor segments. It ensures that each segment is sent to the appropriate motor at the correct time, maintaining the sequence and timing of the pattern.

#### Scheduler Data Structure

The scheduler maintains a queue of segments to be executed:

