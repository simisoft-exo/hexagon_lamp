package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"device_commander/comms"
	"device_commander/lights"
	"device_commander/motors"

	"github.com/gdamore/tcell/v2"
	"github.com/rs/cors"
)

type ScreenUpdate = comms.ScreenUpdate

var (
	screenUpdateChan    = make(chan ScreenUpdate, 100) // Buffer for 100 updates
	screenRefreshTicker *time.Ticker
)

var (
	connections      []*comms.SerialConnection
	connectionsMutex sync.Mutex
	currentPortIndex int
	screen           tcell.Screen
	inputBuffer      string
	logFile          *os.File
	sendToAllBuffer  string
	btBuffer         string
	panel            *lights.HexagonPanel
	currentPattern   *motors.Pattern
)

type DeviceStatus struct {
	LastACK       time.Time
	LastHeartbeat time.Time
}

var (
	deviceStatuses map[string]*DeviceStatus
)

// Initialize LEDs in init() function
func init() {
	var err error
	panel, err = lights.NewHexagonPanel()
	if err != nil {
		log.Fatalf("Error creating hexagon panel: %v", err)
	}
	lights.InitializeLEDs(panel)
	log.Println("Initialized LEDs")
	connections = make([]*comms.SerialConnection, 0, 7)
	deviceStatuses = make(map[string]*DeviceStatus)
}

func main() {

	var err error
	logFile, err = os.OpenFile("debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetPrefix("main: ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if err != nil {
		log.Fatalf("Error creating hexagon panel: %v", err)
	}

	comms.SetScreenUpdateChan(screenUpdateChan)

	// Device info for all 7 devices
	devicesJSON := `[
        {"device_serial_no": "0671FF383159503043112607", "device_id": "0", "serial_port": "/dev/ttyACM0"},
        {"device_serial_no": "066DFF515049657187212124", "device_id": "1", "serial_port": "/dev/ttyACM1"},
        {"device_serial_no": "066CFF383159503043112637", "device_id": "2", "serial_port": "/dev/ttyACM2"},
        {"device_serial_no": "066BFF515049657187203314", "device_id": "3", "serial_port": "/dev/ttyACM3"},
        {"device_serial_no": "066FFF383159503043114308", "device_id": "4", "serial_port": "/dev/ttyACM4"},
        {"device_serial_no": "066EFF383159503043112729", "device_id": "5", "serial_port": "/dev/ttyACM5"},
        {"device_serial_no": "066CFF383159503043112926", "device_id": "6", "serial_port": "/dev/ttyACM6"}
    ]`
	var devices []comms.DeviceInfo
	err = json.Unmarshal([]byte(devicesJSON), &devices)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize device statuses for all devices
	for _, device := range devices {
		deviceStatuses[device.DeviceID] = &DeviceStatus{}
	}

	var wg sync.WaitGroup
	connectionChan := make(chan *comms.SerialConnection, len(devices))

	for _, device := range devices {
		wg.Add(1)
		go func(dev comms.DeviceInfo) {
			defer wg.Done()
			conn, err := comms.OpenSerialPort(dev.SerialPort, connections, &connectionsMutex)
			if err != nil {
				log.Printf("Failed to open %s: %v", dev.SerialPort, err)
				partialConn := &comms.SerialConnection{
					DeviceID: dev.DeviceID,
					PortName: dev.SerialPort,
				}
				go comms.AttemptReconnection(partialConn, connections, &connectionsMutex)
				return
			}
			conn.DeviceID = dev.DeviceID
			conn.PortName = dev.SerialPort
			connectionChan <- conn

			// Start a goroutine to handle updates for this device
			go handleDeviceUpdates(conn)
		}(device)
	}

	go func() {
		wg.Wait()
		close(connectionChan)
	}()

	for conn := range connectionChan {
		connections = append(connections, conn)
		go comms.PeriodicHandshake(conn, connections, &connectionsMutex)
	}

	if len(connections) == 0 {
		log.Fatal("No serial connections were successfully opened")
	}

	screen, err = tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}
	defer screen.Fini()

	// Start the screen update goroutine
	go screenUpdateLoop()

	go comms.RunBluetooth()
	go lights.Run(panel)

	// Start the screen refresh ticker
	screenRefreshTicker = time.NewTicker(100 * time.Millisecond)
	defer screenRefreshTicker.Stop()

	// Start the HTTP server
	go startHTTPServer()

	drawScreen()
	for {
		select {
		case <-screenRefreshTicker.C:
			drawScreen()
		default:
			switch ev := screen.PollEvent().(type) {
			case *tcell.EventResize:
				screen.Sync()
				drawScreen()
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape:
					return
				case tcell.KeyEnter:
					if currentPortIndex == len(connections) {
						if sendToAllBuffer == "PAT" {
							go playCurrentPattern()
						}
						comms.SendCommandToAll(sendToAllBuffer, &connectionsMutex, connections)
						sendToAllBuffer = ""
					} else if currentPortIndex == len(connections)+1 {
						// Send BT buffer to all devices
						comms.SendCommandToAll(btBuffer, &connectionsMutex, connections)
						btBuffer = ""
					} else {
						comms.SendCommand(inputBuffer, &connectionsMutex, connections, currentPortIndex)
						inputBuffer = ""
					}
				case tcell.KeyBackspace, tcell.KeyBackspace2:
					if currentPortIndex == len(connections) {
						if len(sendToAllBuffer) > 0 {
							sendToAllBuffer = sendToAllBuffer[:len(sendToAllBuffer)-1]
						}
					} else if currentPortIndex == len(connections)+1 {
						if len(btBuffer) > 0 {
							btBuffer = btBuffer[:len(btBuffer)-1]
						}
					} else {
						if len(inputBuffer) > 0 {
							inputBuffer = inputBuffer[:len(inputBuffer)-1]
						}
					}
				case tcell.KeyTab:
					if ev.Modifiers() == tcell.ModShift {
						currentPortIndex = (currentPortIndex - 1 + len(connections) + 2) % (len(connections) + 2)
					} else {
						currentPortIndex = (currentPortIndex + 1) % (len(connections) + 2)
					}
				case tcell.KeyBacktab:
					currentPortIndex = (currentPortIndex - 1 + len(connections) + 2) % (len(connections) + 2)
				case tcell.KeyRune:
					if ev.Rune() == 'q' && (ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModMeta) {
						return // Exit when Alt+Q or Option+Q is pressed
					}
					if currentPortIndex == len(connections) {
						sendToAllBuffer += string(ev.Rune())
					} else if currentPortIndex == len(connections)+1 {
						btBuffer += string(ev.Rune())
					} else {
						inputBuffer += string(ev.Rune())
					}
				}
				drawScreen()
			}
		}
	}
}

func drawScreen() {
	if screen == nil {
		log.Println("Screen is nil in drawScreen()")
		return
	}

	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	screen.Clear()
	width, height := screen.Size()

	// Reserve 3 lines for input, 1 for "Send to All", 1 for Bluetooth, and 1 for debug info
	availableHeight := height - 5

	// Sort connections by DeviceID
	sortedConnections := make([]*comms.SerialConnection, len(connections))
	copy(sortedConnections, connections)
	sort.Slice(sortedConnections, func(i, j int) bool {
		return sortedConnections[i].DeviceID < sortedConnections[j].DeviceID
	})

	// Ensure at least 2 lines per device (1 for header, 1 for output)
	deviceHeight := max(2, availableHeight/len(sortedConnections))

	// Draw device info and outputs
	for i, conn := range sortedConnections {
		y := i * deviceHeight
		if y >= availableHeight {
			break
		}

		// Draw device header
		headerText := fmt.Sprintf("Device ID: %s, Port: %s, Output Length: %d", conn.DeviceID, conn.PortName, len(conn.Output))
		if i == currentPortIndex {
			highlightText(0, y, width, headerText, tcell.ColorGreen, tcell.ColorBlack)
		} else {
			drawText(0, y, width, headerText)
		}

		// Draw status line
		status := deviceStatuses[conn.DeviceID]
		if status != nil {
			statusText := fmt.Sprintf("ACK: %s | HEARTBEAT: %s",
				formatTimestamp(status.LastACK),
				formatTimestamp(status.LastHeartbeat))
			drawText(0, y+1, width, statusText)
		}

		// Draw device output
		lines := strings.Split(conn.Output, "\n")
		outputHeight := deviceHeight - 2 // Reserve two lines for header and status
		startLine := max(0, len(lines)-outputHeight)
		for j := 0; j < outputHeight && startLine+j < len(lines); j++ {
			if y+j+2 >= availableHeight {
				break
			}
			line := lines[startLine+j]
			drawText(0, y+j+2, width, line)
		}
	}

	// Draw input line
	inputLine := fmt.Sprintf("%d-> %s", currentPortIndex, inputBuffer)
	drawText(0, height-4, width, inputLine)

	// Draw "Send to All" input line
	sendToAllLine := fmt.Sprintf("Send to All> %s", sendToAllBuffer)
	if currentPortIndex == len(sortedConnections) {
		highlightText(0, height-3, width, sendToAllLine, tcell.ColorGreen, tcell.ColorBlack)
	} else {
		drawText(0, height-3, width, sendToAllLine)
	}

	// Draw Bluetooth data line
	btLine := fmt.Sprintf("BT> %s", btBuffer)
	if currentPortIndex == len(sortedConnections)+1 {
		highlightText(0, height-2, width, btLine, tcell.ColorGreen, tcell.ColorBlack)
	} else {
		drawText(0, height-2, width, btLine)
	}

	// Draw debug info
	debugInfo := fmt.Sprintf("Connected Devices: %d", len(sortedConnections))
	drawText(0, height-1, width, debugInfo)

	screen.Show()
}

func drawText(x, y, maxWidth int, text string) {
	for i, r := range []rune(text) {
		if i >= maxWidth {
			break
		}
		screen.SetContent(x+i, y, r, nil, tcell.StyleDefault)
	}
}

func highlightText(x, y, maxWidth int, text string, fg, bg tcell.Color) {
	style := tcell.StyleDefault.Foreground(fg).Background(bg)
	for i, r := range []rune(text) {
		if i >= maxWidth {
			break
		}
		screen.SetContent(x+i, y, r, nil, style)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const maxOutputLines = 100

func limitOutputBuffer(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > maxOutputLines {
		lines = lines[len(lines)-maxOutputLines:]
	}
	return strings.Join(lines, "\n")
}

// Add this new function at the top level of the file
func logAllDeviceStatuses() {
	log.Println("Current status of all devices:")
	for deviceID, status := range deviceStatuses {
		log.Printf("Device %s - ACK: %s, HEARTBEAT: %s",
			deviceID,
			formatTimestamp(status.LastACK),
			formatTimestamp(status.LastHeartbeat))
	}
}

func screenUpdateLoop() {
	statusCheckTicker := time.NewTicker(10 * time.Second)
	defer statusCheckTicker.Stop()

	for {
		select {
		case update := <-screenUpdateChan:
			if update.DeviceID == "BT" {
				// Handle Bluetooth updates as before
				log.Printf("Received Bluetooth data: %s", update.Output)
				btBuffer = update.Output
				log.Printf("Updated Bluetooth buffer: %s", btBuffer)
			}
		case <-screenRefreshTicker.C:
			drawScreen()
		case <-statusCheckTicker.C:
			logAllDeviceStatuses()
		}
	}
}

// Add this new function
func safeUpdateDeviceStatus(deviceID string, isACK, isHeartbeat bool) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	updateDeviceStatus(deviceID, isACK, isHeartbeat)
}

// Modify the processDeviceUpdate function
func processDeviceUpdate(update ScreenUpdate) {
	for _, conn := range connections {
		if conn.DeviceID == update.DeviceID {
			if strings.Contains(update.Output, "ACK") {
				safeUpdateDeviceStatus(conn.DeviceID, true, false)
			}
			if strings.Contains(update.Output, "HEARTBEAT") {
				safeUpdateDeviceStatus(conn.DeviceID, false, true)
			}

			filteredOutput := filterAckHeartbeat(update.Output)
			if filteredOutput != "" {
				connectionsMutex.Lock()
				conn.Output = limitOutputBuffer(conn.Output + filteredOutput)
				connectionsMutex.Unlock()
			}
			break
		}
	}
}

// Add this new function to filter out ACK and HEARTBEAT messages
func filterAckHeartbeat(output string) string {
	lines := strings.Split(output, "\n")
	var filteredLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "ACK" && trimmedLine != "HEARTBEAT" {
			filteredLines = append(filteredLines, line)
		}
	}
	return strings.Join(filteredLines, "\n")
}

func startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/pattern", handlePattern)
	mux.HandleFunc("/serial-numbers", handleSerialNumbers) // Add this line

	// Create a new CORS handler
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // Allow all origins
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"},
	})

	// Wrap your mux with the CORS handler
	handler := c.Handler(mux)

	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func handlePattern(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received pattern request from %s", r.RemoteAddr)
	if r.Method != http.MethodPost {
		log.Printf("Invalid method for pattern request: %s", r.Method)
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading pattern request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Log the received JSON
	log.Printf("Received pattern JSON: %s", string(body))

	var pattern motors.Pattern
	err = json.Unmarshal(body, &pattern)
	if err != nil {
		log.Printf("Error decoding pattern JSON: %v", err)
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	currentPattern = &pattern
	log.Printf("New pattern set: %+v", pattern)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Pattern received successfully"))
	log.Printf("Pattern request processed successfully")
}

func playCurrentPattern() {
	if currentPattern == nil {
		log.Println("No pattern to play")
		return
	}

	err := motors.ScheduleMotorMovements(currentPattern, &connectionsMutex, connections)
	if err != nil {
		log.Printf("Error playing pattern: %v", err)
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("15:04:05")
}

func updateDeviceStatus(deviceID string, isACK, isHeartbeat bool) {
	status, exists := deviceStatuses[deviceID]
	if !exists {
		status = &DeviceStatus{}
		deviceStatuses[deviceID] = status
		log.Printf("Created new status for device %s", deviceID)
	}

	now := time.Now()
	if isACK {
		status.LastACK = now
		log.Printf("Updated ACK for device %s: %s", deviceID, now.Format("15:04:05"))
	}
	if isHeartbeat {
		status.LastHeartbeat = now
		log.Printf("Updated HEARTBEAT for device %s: %s", deviceID, now.Format("15:04:05"))
	}
}

// Modify the handleDeviceUpdates function
func handleDeviceUpdates(conn *comms.SerialConnection) {
	deviceUpdateChan := make(chan comms.ScreenUpdate, 100)
	go comms.ReadSerialOutput(conn, deviceUpdateChan, connections, &connectionsMutex)

	for update := range deviceUpdateChan {
		go processDeviceUpdate(update)
	}
}

func handleSerialNumbers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received serial numbers request from %s", r.RemoteAddr)
	if r.Method != http.MethodGet {
		log.Printf("Invalid method for serial numbers request: %s", r.Method)
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create a channel for the result
	resultChan := make(chan map[string]map[string]string)

	// Run getSerialNumbers in a goroutine
	go func() {
		resultChan <- getSerialNumbers()
	}()

	// Wait for the result or timeout
	select {
	case serialNumbers := <-resultChan:
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(serialNumbers); err != nil {
			log.Printf("Error encoding serial numbers: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		log.Printf("Serial numbers request processed successfully")
	case <-time.After(15 * time.Second):
		log.Printf("Timeout while processing serial numbers request")
		http.Error(w, "Request timed out", http.StatusRequestTimeout)
	}
}

// Modify the getSerialNumbers function
func getSerialNumbers() map[string]map[string]string {
	log.Println("Collecting serial numbers for all devices")
	result := make(map[string]map[string]string)
	var wg sync.WaitGroup
	resultMutex := sync.Mutex{}

	for _, conn := range connections {
		wg.Add(1)
		go func(conn *comms.SerialConnection) {
			defer wg.Done()
			deviceInfo := make(map[string]string)
			deviceInfo["hardcoded_serial"] = getHardcodedSerialNumber(conn.DeviceID)
			deviceInfo["received_serial"] = ""

			log.Printf("Sending 'S' command to device %s", conn.DeviceID)
			comms.SendCommand("S", &connectionsMutex, connections, getDeviceIndex(conn.DeviceID))

			// Wait and check for the serial number multiple times
			for attempt := 0; attempt < 5; attempt++ {
				time.Sleep(500 * time.Millisecond)

				connectionsMutex.Lock()
				output := conn.Output
				connectionsMutex.Unlock()

				log.Printf("Raw output received from device %s (attempt %d):\n%s", conn.DeviceID, attempt+1, output)

				serialNo := extractSerialNumber(output)
				if serialNo != "" {
					deviceInfo["received_serial"] = serialNo
					log.Printf("Parsed serial number for device %s: %s", conn.DeviceID, serialNo)
					break
				}

				if attempt == 4 {
					log.Printf("Failed to retrieve serial number for device %s after 5 attempts", conn.DeviceID)
				}
			}

			resultMutex.Lock()
			result[conn.DeviceID] = deviceInfo
			resultMutex.Unlock()
		}(conn)
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		log.Println("Finished collecting serial numbers")
	case <-time.After(10 * time.Second):
		log.Println("Timeout while collecting serial numbers")
	}

	return result
}

func extractSerialNumber(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "SERIAL_NO:") && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1])
		}
	}
	return ""
}

func getHardcodedSerialNumber(deviceID string) string {
	// This function should return the hardcoded serial number for the given device ID
	// You'll need to implement this based on your device information
	hardcodedSerials := map[string]string{
		"0": "0671FF383159503043112607",
		"1": "066DFF515049657187212124",
		"2": "066CFF383159503043112637",
		"3": "066BFF515049657187203314",
		"4": "066FFF383159503043114308",
		"5": "066EFF383159503043112729",
		"6": "066CFF383159503043112926",
	}
	return hardcodedSerials[deviceID]
}

func getDeviceIndex(deviceID string) int {
	for i, conn := range connections {
		if conn.DeviceID == deviceID {
			return i
		}
	}
	return -1
}
