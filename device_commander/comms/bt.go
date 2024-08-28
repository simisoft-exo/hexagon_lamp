package comms

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
	"github.com/go-ble/ble/linux/hci/evt"
	"github.com/pkg/errors"
)

var screenUpdateChan chan ScreenUpdate

func SetScreenUpdateChan(ch chan ScreenUpdate) {
	screenUpdateChan = ch
}

var (
	connectedDevices map[string]ble.Client
	deviceMutex      sync.Mutex
)

func init() {
	connectedDevices = make(map[string]ble.Client)
}

func handleDisconnect(c ble.Client) {
	deviceMutex.Lock()
	defer deviceMutex.Unlock()

	// Remove the disconnected device from our map
	for addr, client := range connectedDevices {
		if client == c {
			delete(connectedDevices, addr)
			log.Printf("bt: Device %s disconnected", addr)

			// Attempt to reconnect
			go func(address string) {
				for {
					log.Printf("bt: Attempting to reconnect to %s", address)
					ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), 30*time.Second))
					c, err := ble.Connect(ctx, filter(address))
					if err != nil {
						log.Printf("bt: Failed to reconnect to %s: %v", address, err)
						time.Sleep(5 * time.Second)
						continue
					}

					deviceMutex.Lock()
					connectedDevices[address] = c
					deviceMutex.Unlock()

					log.Printf("bt: Successfully reconnected to %s", address)
					return
				}
			}(addr)

			break
		}
	}
}

func filter(addr string) ble.AdvFilter {
	return func(a ble.Advertisement) bool {
		return a.Addr().String() == addr
	}
}

func RunBluetooth() {
	// Open bt-debug.log file for logging in the current directory
	debugFile, err := os.OpenFile("bt-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("bt: Can't open bt-debug.log file: %s", err)
	}
	defer debugFile.Close()

	// Create a new logger for this package
	btLogger := log.New(debugFile, "bt: ", log.LstdFlags)

	// Set the output of the default logger to the debug file
	log.SetOutput(debugFile)

	// Use the package-specific logger
	log.SetOutput(btLogger.Writer())

	d, err := dev.NewDevice("Hexagon")
	if err != nil {
		log.Fatalf("bt: Can't create device : %s", err)
	}
	ble.SetDefaultDevice(d)

	// Log the device MAC address
	log.Printf("bt: Device Info uuid: %s", ble.DeviceInfoUUID.String())
	log.Printf("bt: Device Name uuid: %s", ble.DeviceNameUUID.String())

	// Define a characteristic for receiving data
	rxChar := ble.NewCharacteristic(ble.MustParse("19B10001-E8F2-537E-4F6C-D104768A1214"))
	rxChar.HandleWrite(
		ble.WriteHandlerFunc(func(req ble.Request, rsp ble.ResponseWriter) {
			data := req.Data()
			if len(data) > 0 {
				log.Printf("bt: Received raw data: %v", data)
				log.Printf("bt: Received string data: %s", string(data))
				log.Printf("bt: Received data: %s", string(data))

				// Send the received string data to the main screen drawing
				if screenUpdateChan != nil {
					select {
					case screenUpdateChan <- ScreenUpdate{
						DeviceID: "BT",
						Output:   string(data),
					}:
					default:
						log.Println("bt: Failed to send update, channel full")
					}
				} else {
					log.Println("bt: screenUpdateChan is nil")
				}
			}
		}),
	)

	// Add the characteristic to a service
	svc := ble.NewService(ble.MustParse("19B10000-E8F2-537E-4F6C-D104768A1214"))
	svc.AddCharacteristic(rxChar)

	// Add the service to the device
	if err := ble.AddService(svc); err != nil {
		log.Fatalf("bt: Can't add service: %s", err)
	}

	// Start advertising
	advertise := func() {
		ctx := ble.WithSigHandler(context.Background(), nil)
		chkErr(ble.AdvertiseNameAndServices(ctx, "Hexagon"))
	}

	advertise()

	// Handle connections
	ble.OptConnectHandler(func(evt evt.LEConnectionComplete) {
		addr := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			evt.PeerAddress()[5], evt.PeerAddress()[4], evt.PeerAddress()[3],
			evt.PeerAddress()[2], evt.PeerAddress()[1], evt.PeerAddress()[0])
		log.Printf("bt: OptConnectHandler called for address %s", addr)

		ctx := context.Background()
		c, err := ble.Connect(ctx, filter(addr))
		if err != nil {
			log.Printf("bt: Failed to connect to %s: %v", addr, err)
			return
		}

		log.Printf("bt: Successfully connected to %s", addr)

		deviceMutex.Lock()
		connectedDevices[addr] = c
		deviceMutex.Unlock()

		log.Printf("bt: Added %s to connectedDevices", addr)

		go func() {
			<-c.Disconnected()
			log.Printf("bt: Disconnection detected for %s", addr)
			handleDisconnect(c)
		}()

		if screenUpdateChan != nil {
			select {
			case screenUpdateChan <- ScreenUpdate{
				DeviceID: "BT",
				Output:   "Connected to " + addr,
			}:
				log.Printf("bt: Sent connection update for %s", addr)
			default:
				log.Printf("bt: Failed to send connection update for %s, channel full", addr)
			}
		} else {
			log.Printf("bt: screenUpdateChan is nil for ble connection to %s", addr)
		}

		log.Printf("bt: OptConnectHandler completed for %s", addr)
	})

	// Handle disconnections
	ble.OptDisconnectHandler(func(evt evt.DisconnectionComplete) {
		log.Printf("bt: Disconnected from %s", strconv.Itoa(int(evt.ConnectionHandle())))
		if screenUpdateChan != nil {
			select {
			case screenUpdateChan <- ScreenUpdate{
				DeviceID: "BT",
				Output:   "Disconnected from " + strconv.Itoa(int(evt.ConnectionHandle())),
			}:
			default:
				log.Println("bt: Failed to send disconnection update, channel full")
			}
		} else {
			log.Println("bt: screenUpdateChan is nil for ble disconnection")
		}
		// Start advertising again after disconnection
		go advertise()
	})

	// Keep the function running
	select {}
}

func chkErr(err error) {
	switch errors.Cause(err) {
	case nil:
	case context.DeadlineExceeded:
		log.Printf("bt: timeout deadline exceeded\n")
	case context.Canceled:
		log.Printf("bt:canceled\n")
	default:
		log.Fatal(err)
	}
}
