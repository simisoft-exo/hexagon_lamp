#!/bin/bash

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

# Navigate to the sketch directory
log "Changing to sketch directory"
cd "$SKETCH_DIR" || { log "Error: Unable to find $SKETCH_NAME directory"; exit 1; }
log "Current directory: $(pwd)"

# Compile the Arduino sketch with caching, parallel jobs, and size optimization
log "Starting Arduino CLI compilation..."
arduino-cli compile -b $BOARD $SKETCH_NAME.ino \
    --build-path "$BUILD_PATH" \
    --build-cache-path "$BUILD_PATH/cache" \
    --log-level info

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
