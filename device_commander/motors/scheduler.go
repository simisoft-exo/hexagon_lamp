package motors

import (
	"device_commander/comms"
	"fmt"
	"sync"
	"time"
)

type Segment struct {
	Duration int     `json:"duration"`
	Speed    float64 `json:"speed"`
}

type Pattern struct {
	Patterns struct {
		MotorId  int       `json:"motorId"`
		Segments []Segment `json:"segments"`
	} `json:"patterns"`
}

func ScheduleMotorMovements(pattern *Pattern, connectionsMutex *sync.Mutex, connections []*comms.SerialConnection) error {
	if pattern == nil {
		return fmt.Errorf("pattern is nil")
	}

	startTime := time.Now()

	go func() {
		var elapsedTime time.Duration
		for _, segment := range pattern.Patterns.Segments {
			// Calculate when this segment should start
			segmentStartTime := startTime.Add(elapsedTime)

			// Wait until it's time to execute this segment
			time.Sleep(time.Until(segmentStartTime))

			// Send command to move motor
			err := MoveMotor(int64(pattern.Patterns.MotorId), float64(segment.Speed), connectionsMutex, connections)
			if err != nil {
				fmt.Printf("Error moving motor %d: %v\n", pattern.Patterns.MotorId, err)
			}

			// Update elapsed time
			elapsedTime += time.Duration(segment.Duration) * time.Millisecond
		}
	}()

	return nil
}

func MoveMotor(motorId int64, velocity float64, connectionsMutex *sync.Mutex, connections []*comms.SerialConnection) error {
	command := fmt.Sprintf("M%.2f", velocity)
	currentPortIndex := int(motorId) // Assuming motorId corresponds to the port index

	if currentPortIndex < 0 || currentPortIndex >= len(connections) {
		return fmt.Errorf("invalid motor ID: %d", motorId)
	}

	comms.SendCommand(command, connectionsMutex, connections, currentPortIndex)
	return nil
}

// func main() {
// 	filename := "pink_panther.json" // Assuming the JSON file is in the same directory
// 	err := ScheduleMotorMovements(filename)
// 	if err != nil {
// 		fmt.Printf("Error scheduling motor movements: %v\n", err)
// 		return
// 	}

// 	// Wait for all goroutines to finish
// 	// This is a simple way to keep the main function running
// 	// You might want to implement a more sophisticated wait mechanism in a real application
// 	fmt.Println("Motor movements scheduled. Press Ctrl+C to exit.")
// 	select {}
// }
