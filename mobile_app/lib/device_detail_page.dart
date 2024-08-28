import 'package:flutter/material.dart';
import 'package:flutter_blue_plus/flutter_blue_plus.dart';

class DeviceDetailPage extends StatefulWidget {
  final BluetoothDevice device;

  const DeviceDetailPage({required this.device});

  @override
  DeviceDetailPageState createState() => DeviceDetailPageState();
}

class DeviceDetailPageState extends State<DeviceDetailPage> {
  List<BluetoothService> services = [];

  @override
  void initState() {
    super.initState();
    discoverServices();
  }

  void discoverServices() async {
    services = await widget.device.discoverServices();
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.device.platformName ?? 'Unknown Device'),
      ),
      body: ListView.builder(
        itemCount: services.length,
        itemBuilder: (context, index) {
          BluetoothService service = services[index];
          return ExpansionTile(
            title: Text('Service: ${service.uuid}'),
            children: service.characteristics.map((c) {
              return ExpansionTile(
                title: Text('Characteristic: ${c.uuid}'),
                subtitle: Text('Properties: ${c.properties}'),
                children: [
                  if (c.properties.read)
                    ListTile(
                      leading: const Icon(Icons.visibility),
                      title: const Text('Read'),
                      onTap: () => readCharacteristic(c),
                    ),
                  if (c.properties.write)
                    ListTile(
                      leading: const Icon(Icons.edit),
                      title: const Text('Write'),
                      onTap: () => writeCharacteristic(c),
                    ),
                  if (c.properties.notify)
                    ListTile(
                      leading: const Icon(Icons.notifications),
                      title: const Text('Notify'),
                      onTap: () => subscribeToCharacteristic(c),
                    ),
                ],
              );
            }).toList(),
          );
        },
      ),
    );
  }

  void readCharacteristic(BluetoothCharacteristic c) async {
    try {
      List<int> value = await c.read();
      print('Read value: $value');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Read value: $value')),
        );
      }
    } catch (e) {
      print('Error reading characteristic: $e');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error reading characteristic: $e')),
        );
      }
    }
  }

  void writeCharacteristic(BluetoothCharacteristic c) async {
    try {
      // Show a dialog to get user input
      String? userInput = await showDialog<String>(
        context: context,
        builder: (BuildContext context) {
          String inputText = '';
          return AlertDialog(
            title: const Text('Write to Characteristic'),
            content: TextField(
              onChanged: (value) {
                inputText = value;
              },
              decoration: const InputDecoration(hintText: "Enter data to write"),
            ),
            actions: <Widget>[
              TextButton(
                child: const Text('Cancel'),
                onPressed: () {
                  Navigator.of(context).pop();
                },
              ),
              TextButton(
                child: const Text('Write'),
                onPressed: () {
                  Navigator.of(context).pop(inputText);
                },
              ),
            ],
          );
        },
      );

      if (userInput != null && userInput.isNotEmpty) {
        List<int> bytes = userInput.codeUnits;
        await c.write(bytes);
        print('Write successful: $userInput');
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Write successful: $userInput')),
          );
        }
      } else {
        print('Write cancelled or empty input');
      }
    } catch (e) {
      print('Error writing characteristic: $e');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error writing characteristic: $e')),
        );
      }
    }
  }

  void subscribeToCharacteristic(BluetoothCharacteristic c) async {
    try {
      await c.setNotifyValue(true);
      c.lastValueStream.listen((value) {
        print('Notification received: $value');
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Notification received: $value')),
          );
        }
      });
    } catch (e) {
      print('Error subscribing to characteristic: $e');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error subscribing to characteristic: $e')),
        );
      }
    }
  }
}
