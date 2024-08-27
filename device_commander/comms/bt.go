package comms

import (
	"context"
	"log"
	"os"
	"strconv"
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

func RunBluetooth() {
	// Open debug.log file for logging
	debugFile, err := os.OpenFile("bt-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("bt: Can't open debug.log file: %s", err)
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
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), 300*time.Second))
	chkErr(ble.AdvertiseNameAndServices(ctx, "Hexagon"))

	// Handle connections

	ble.OptConnectHandler(func(evt evt.LEConnectionComplete) {
		log.Printf("bt: Got connection from %s", strconv.Itoa(int(evt.ConnectionHandle())))
		if screenUpdateChan != nil {
			select {
			case screenUpdateChan <- ScreenUpdate{
				DeviceID: "BT",
				Output:   "Connected to " + strconv.Itoa(int(evt.ConnectionHandle())),
			}:
			default:
				log.Println("bt: Failed to send connection update, channel full")
			}
		} else {
			log.Println("bt: screenUpdateChan is nil for ble connection")
			log.Println("bt: Got connection from " + strconv.Itoa(int(evt.ConnectionHandle())))
		}
	})

	for range ctx.Done() {
		log.Println("bt: Context done, restarting advertising")
		// Restart advertising
		ctx = ble.WithSigHandler(context.WithTimeout(context.Background(), 300*time.Second))
		err := ble.AdvertiseNameAndServices(ctx, "Hexagon")
		if err != nil {
			log.Printf("bt: Failed to restart advertising: %v", err)
		}
	}
}

func chkErr(err error) {
	switch errors.Cause(err) {
	case nil:
	case context.DeadlineExceeded:
		log.Printf("bt:done\n")
	case context.Canceled:
		log.Printf("bt:canceled\n")
	default:
		log.Fatal(err)
	}
}
