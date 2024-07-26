#!/bin/bash

# Read the JSON file and extract serial numbers
serial_numbers=$(jq -r '.[].device_serial_no' device_mappings.json)

# Loop through each serial number
for serial in $serial_numbers
do
  echo "Uploading to device with serial number $serial"
  
  # Use st-flash with the specific device serial number
  st-flash --serial $serial write /tmp/arduino/sketches/9966A1D8593F74EE345AA9DA5FF68892/simplefoc_tuning.ino.bin 0x08000000

  if [ $? -eq 0 ]; then
    echo "Upload successful for device with serial number $serial"
  else
    echo "Error: Upload failed for device with serial number $serial"
  fi

  echo "----------------------------"
  sleep 5  # Add a 5-second delay between upload attempts
done