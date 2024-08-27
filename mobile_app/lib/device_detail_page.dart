import 'package:flutter/material.dart';
import 'package:flutter_blue_plus/flutter_blue_plus.dart';

class DeviceDetailPage extends StatefulWidget {
  final BluetoothDevice device;

  DeviceDetailPage({required this.device});

  @override
  _DeviceDetailPageState createState() => _DeviceDetailPageState();
}

class _DeviceDetailPageState extends State<DeviceDetailPage> {
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
        title: Text(widget.device.name ?? 'Unknown Device'),
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
                      leading: Icon(Icons.visibility),
                      title: Text('Read'),
                      onTap: () => readCharacteristic(c),
                    ),
                  if (c.properties.write)
                    ListTile(
                      leading: Icon(Icons.edit),
                      title: Text('Write'),
                      onTap: () => writeCharacteristic(c),
                    ),
                  if (c.properties.notify)
                    ListTile(
                      leading: Icon(Icons.notifications),
                      title: Text('Notify'),
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
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Read value: $value')),
      );
    } catch (e) {
      print('Error reading characteristic: $e');
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Error reading characteristic: $e')),
      );
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
            title: Text('Write to Characteristic'),
            content: TextField(
              onChanged: (value) {
                inputText = value;
              },
              decoration: InputDecoration(hintText: "Enter data to write"),
            ),
            actions: <Widget>[
              TextButton(
                child: Text('Cancel'),
                onPressed: () {
                  Navigator.of(context).pop();
                },
              ),
              TextButton(
                child: Text('Write'),
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Write successful: $userInput')),
        );
      } else {
        print('Write cancelled or empty input');
      }
    } catch (e) {
      print('Error writing characteristic: $e');
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Error writing characteristic: $e')),
      );
    }
  }

  void subscribeToCharacteristic(BluetoothCharacteristic c) async {
    try {
      await c.setNotifyValue(true);
      c.value.listen((value) {
        print('Notification received: $value');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Notification received: $value')),
        );
      });
    } catch (e) {
      print('Error subscribing to characteristic: $e');
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Error subscribing to characteristic: $e')),
      );
    }
  }
}
