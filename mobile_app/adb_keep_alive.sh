#!/bin/bash

# ADB Keep Alive Script for device adb-7dc64e52-k2kMot._adb-tls-connect._tcp

DEVICE="7dc64e52-k2kMot._adb-tls-connect._tcp"

# Function to check ADB connection
check_adb_connection() {
    adb devices | grep -q "$DEVICE"
}

# Function to send a harmless ADB command to keep the connection alive
keep_alive() {
    adb -s "$DEVICE" shell echo "Keeping connection alive" > /dev/null 2>&1
}

# Main loop
while true; do
    if check_adb_connection; then
        echo "ADB connection is alive. Sending keep-alive signal."
        keep_alive
    else
        echo "ADB connection lost. Attempting to reconnect..."
        adb connect "$DEVICE"
    fi
    
    # Wait for 60 seconds before checking again
    sleep 60
done

