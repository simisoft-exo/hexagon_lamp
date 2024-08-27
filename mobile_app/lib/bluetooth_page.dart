import 'package:flutter/material.dart';
import 'package:flutter_blue_plus/flutter_blue_plus.dart';
import 'package:permission_handler/permission_handler.dart';
import 'device_detail_page.dart'; 

class BluetoothPage extends StatefulWidget {
  @override
  _BluetoothPageState createState() => _BluetoothPageState();
}

class _BluetoothPageState extends State<BluetoothPage> {
  List<ScanResult> scanResults = [];
  bool isScanning = false;
  bool isBluetoothEnabled = false;

  @override
  void initState() {
    super.initState();
    checkBluetoothStatus();
  }

  Future<void> checkBluetoothStatus() async {
    bool isEnabled = await FlutterBluePlus.isOn;
    setState(() {
      isBluetoothEnabled = isEnabled;
    });
    if (isBluetoothEnabled) {
      checkPermissions();
    }
  }

  Future<void> checkPermissions() async {
    Map<Permission, PermissionStatus> statuses = await [
      Permission.bluetooth,
      Permission.location,
    ].request();

    if (statuses[Permission.bluetooth]!.isGranted &&
        statuses[Permission.location]!.isGranted) {
      startScan();
    } else {
      // Handle the case where permissions are not granted
      print("Permissions not granted");
    }
  }

  void startScan() async {
    setState(() {
      isScanning = true;
      scanResults.clear();
    });

    try {
      await FlutterBluePlus.startScan(timeout: Duration(seconds: 4));
      FlutterBluePlus.scanResults.listen((results) {
        setState(() {
          scanResults = results;
        });
      });
    } catch (e) {
      print('Error starting scan: $e');
    }

    setState(() {
      isScanning = false;
    });
  }

  Future<void> connectToDevice(BluetoothDevice device) async {
    try {
      await device.connect();
      print('Connected to ${device.name}');

      List<BluetoothService> services = await device.discoverServices();
      services.forEach((service) {
        print('Service: ${service.uuid}');
        service.characteristics.forEach((characteristic) {
          print('  Characteristic: ${characteristic.uuid}');
        });
      });
    } catch (e) {
      print('Error connecting to device: $e');
    }
  }

  Future<void> enableBluetooth() async {
    if (!isBluetoothEnabled) {
      try {
        await FlutterBluePlus.turnOn();
        await checkBluetoothStatus();
      } catch (e) {
        print('Error enabling Bluetooth: $e');
        // Show a dialog or snackbar to inform the user that Bluetooth couldn't be enabled
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to enable Bluetooth. Please enable it manually.')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Bluetooth Devices'),
      ),
      body: !isBluetoothEnabled
          ? Center(
              child: ElevatedButton(
                onPressed: enableBluetooth,
                child: Text('Enable Bluetooth'),
              ),
            )
          : ListView.builder(
              itemCount: scanResults.where((result) => result.advertisementData.connectable).length,
              itemBuilder: (context, index) {
                ScanResult result = scanResults.where((result) => result.advertisementData.connectable).toList()[index];
                BluetoothDevice device = result.device;
                String deviceName = device.name.isNotEmpty ? device.name : result.advertisementData.localName ?? 'Unknown Device';
                bool isConnected = device.isConnected;
                
                // Calculate signal strength percentage
                int rssi = result.rssi;
                int rssiMin = -100; // Minimum RSSI value (weakest signal)
                int rssiMax = -30;  // Maximum RSSI value (strongest signal)
                double signalPercentage = ((rssi - rssiMin) / (rssiMax - rssiMin) * 100).clamp(0, 100);
                
                return ListTile(
                  title: Text(deviceName),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('UUID: ${device.id.toString()}'),
                      Text('Signal Strength: ${signalPercentage.toStringAsFixed(1)}%'),
                      Text('Status: ${isConnected ? 'Connected' : 'Not Connected'}'),
                    ],
                  ),
                  onTap: () {
                    if (isConnected) {
                      Navigator.push(
                        context,
                        MaterialPageRoute(
                          builder: (context) => DeviceDetailPage(device: device),
                        ),
                      );
                    } else {
                      showDialog(
                        context: context,
                        barrierDismissible: false,
                        builder: (BuildContext context) {
                          return AlertDialog(
                            content: Row(
                              children: [
                                CircularProgressIndicator(),
                                SizedBox(width: 20),
                                Text("Connecting..."),
                              ],
                            ),
                          );
                        },
                      );
                      
                      connectToDevice(device).then((_) {
                        Navigator.of(context).pop(); // Dismiss the dialog
                        Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (context) => DeviceDetailPage(device: device),
                          ),
                        );
                      }).catchError((error) {
                        Navigator.of(context).pop(); // Dismiss the dialog
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('Failed to connect: $error')),
                        );
                      });
                      connectToDevice(device);
                    }
                  },
                  trailing: isConnected ? Icon(Icons.bluetooth_connected) : null,
                );
              },
            ),
      floatingActionButton: isBluetoothEnabled
          ? FloatingActionButton(
              onPressed: isScanning ? null : startScan,
              child: Icon(isScanning ? Icons.stop : Icons.search),
            )
          : null,
    );
  }
}
