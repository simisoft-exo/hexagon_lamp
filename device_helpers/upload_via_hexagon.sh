#!/bin/bash

# Get the absolute path of the script and project root
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
DEVICE_HELPERS="$(dirname "$SCRIPT_PATH")"
PROJECT_ROOT="$(dirname "$DEVICE_HELPERS")"
CURRENT_DIR="$(pwd)"

# Ensure we're running from project root
if [ "$CURRENT_DIR" != "$PROJECT_ROOT" ]; then
    echo "Error: Script must be run from project root directory ($PROJECT_ROOT)"
    echo "Current directory: $CURRENT_DIR"
    echo "Please run as: ./device_helpers/upload_via_hexagon.sh"
    exit 1
fi

# Set variables
SKETCH_NAME="simplefoc_tuning"
BOARD="STMicroelectronics:stm32:Disco"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PARENT_DIR="$( dirname "$SCRIPT_DIR" )"
SKETCH_DIR="$PARENT_DIR/$SKETCH_NAME"
BUILD_PATH="$SCRIPT_DIR/build"
REMOTE_PATH="/home/hexagon/repos/hexagon_lamp/simplefoc_tuning_upload_to_device"
REMOTE_ALIAS="hexagon"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "Script directory: $SCRIPT_DIR"
log "Parent directory: $PARENT_DIR"
log "Sketch directory: $SKETCH_DIR"
log "Build path: $BUILD_PATH"

# Ensure the build directory exists
mkdir -p "$BUILD_PATH"
log "Created build directory (if it didn't exist)"

ARDUINO_CLI_ABS="$DEVICE_HELPERS/bin/arduino-cli"

log "Using arduino-cli at: $ARDUINO_CLI_ABS"
log "Sketch directory: $SKETCH_DIR"

# Verify arduino-cli exists and is executable
if [ ! -x "$ARDUINO_CLI_ABS" ]; then
    if [ ! -e "$ARDUINO_CLI_ABS" ]; then
        log "File does not exist: $ARDUINO_CLI_ABS"
    elif [ ! -f "$ARDUINO_CLI_ABS" ]; then
        log "Path exists but is not a file: $ARDUINO_CLI_ABS"
    else
        log "File exists but is not executable: $ARDUINO_CLI_ABS"
    fi
    log "Error: arduino-cli not found at $ARDUINO_CLI_ABS or is not executable"
    exit 1
fi

# Verify the file exists and is executable
ls -l "$ARDUINO_CLI_ABS" || log "Cannot list arduino-cli file"

# Compile the Arduino sketch with caching, parallel jobs, and size optimization
log "Starting Arduino CLI compilation..."
"$ARDUINO_CLI_ABS" compile -b $BOARD "$SKETCH_DIR/$SKETCH_NAME.ino" \
    --build-path "$BUILD_PATH" \
    --build-cache-path "$BUILD_PATH/cache" \
    --log-level info \
    --jobs $(( $(nproc) - 2 ))

# Check if compilation was successful
if [ $? -eq 0 ]; then
    log "Compilation successful."
    log "Copying binary to remote host..."

    # Copy the binary file to the remote host
    scp "$BUILD_PATH/$SKETCH_NAME.ino.bin" $REMOTE_ALIAS:$REMOTE_PATH

    # Check if file transfer was successful
    if [ $? -eq 0 ]; then
        log "Binary file successfully copied to remote host."
    else
        log "Error: Failed to copy binary file to remote host."
        exit 1
    fi
else
    log "Error: Compilation failed."
    exit 1
fi
