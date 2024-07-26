package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type DeviceMapping struct {
	DeviceID       string `json:"device_id"`
	DeviceSerialNo string `json:"device_serial_no"`
}

type DeviceInfo struct {
	DeviceSerialNo string `json:"device_serial_no"`
	DeviceID       string `json:"device_id"`
	SerialPort     string `json:"serial_port"`
}

func main() {
	// Device mappings (you can replace this with your actual JSON string)
	deviceMappingsJSON := `[
		{"device_id": "0", "device_serial_no": "0671FF383159503043112607"},
		{"device_id": "1", "device_serial_no": "066DFF515049657187212124"},
		{"device_id": "2", "device_serial_no": "066CFF383159503043112637"},
		{"device_id": "3", "device_serial_no": "066BFF515049657187203314"},
		{"device_id": "4", "device_serial_no": "066FFF383159503043114308"},
		{"device_id": "5", "device_serial_no": "066EFF383159503043112729"},
		{"device_id": "6", "device_serial_no": "066CFF383159503043112926"}
	]`

	deviceMappings := parseDeviceMappings(deviceMappingsJSON)
	ttyMappings := getTTYMappings()
	usbDevices := getUSBDevices()

	result := combineInfo(deviceMappings, ttyMappings, usbDevices)

	// Print the result
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonResult))
}

func parseDeviceMappings(jsonStr string) map[string]string {
	var mappings []DeviceMapping
	json.Unmarshal([]byte(jsonStr), &mappings)

	result := make(map[string]string)
	for _, mapping := range mappings {
		result[mapping.DeviceSerialNo] = mapping.DeviceID
	}
	return result
}

func getTTYMappings() map[string]string {
	cmd := exec.Command("bash", "-c", `
		for ttyacm in /dev/ttyACM*; do
			if [ -e "$ttyacm" ]; then
				serial=$(udevadm info -n $ttyacm | grep ID_SERIAL_SHORT | cut -d= -f2)
				echo "$serial:$ttyacm"
			fi
		done
	`)
	output, _ := cmd.Output()

	result := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func getUSBDevices() []map[string]string {
	cmd := exec.Command("lsusb", "-v")
	output, _ := cmd.Output()

	devices := []map[string]string{}
	currentDevice := map[string]string{}

	reSerial := regexp.MustCompile(`iSerial\s+\d+\s+(\S+)`)
	reProduct := regexp.MustCompile(`iProduct\s+\d+\s+(.+)`)

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Bus ") {
			if len(currentDevice) > 0 {
				devices = append(devices, currentDevice)
			}
			currentDevice = map[string]string{}
		}

		if matches := reSerial.FindStringSubmatch(line); len(matches) > 1 {
			currentDevice["iSerial"] = matches[1]
		}

		if matches := reProduct.FindStringSubmatch(line); len(matches) > 1 && strings.Contains(matches[1], "STM32") {
			currentDevice["iProduct"] = matches[1]
		}
	}

	if len(currentDevice) > 0 {
		devices = append(devices, currentDevice)
	}

	return devices
}

func combineInfo(deviceMappings map[string]string, ttyMappings map[string]string, usbDevices []map[string]string) []DeviceInfo {
	var result []DeviceInfo

	for _, device := range usbDevices {
		if serial, ok := device["iSerial"]; ok {
			if deviceID, ok := deviceMappings[serial]; ok {
				if ttyPort, ok := ttyMappings[serial]; ok {
					result = append(result, DeviceInfo{
						DeviceSerialNo: serial,
						DeviceID:       deviceID,
						SerialPort:     ttyPort,
					})
				}
			}
		}
	}

	return result
}
