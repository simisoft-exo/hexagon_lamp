#!/bin/bash

# Function to get ttyACM information
get_ttyacm_info() {
    for ttyacm in /dev/ttyACM*; do
        if [ -e "$ttyacm" ]; then
            serial=$(udevadm info -n $ttyacm | grep ID_SERIAL_SHORT | cut -d= -f2)
            echo "$ttyacm:$serial"
        fi
    done
}

# Store ttyACM info in an awk array
TTYACM_INFO=$(get_ttyacm_info)

# Read device mappings
DEVICE_MAPPINGS=$(cat device_mappings.json)

lsusb -v 2>/dev/null | awk -v ttyacm_info="$TTYACM_INFO" -v device_mappings="$DEVICE_MAPPINGS" '
BEGIN { 
    split("", props)
    split(ttyacm_info, ttyacm_array, "\n")
    for (i in ttyacm_array) {
        split(ttyacm_array[i], temp, ":")
        ttyacm_map[temp[2]] = temp[1]
    }

    n = split(device_mappings, dm_array, /\n/)
    in_array = 0
    for (i = 1; i <= n; i++) {
        if (dm_array[i] ~ /"device_id"/) {
            match(dm_array[i], /"device_id": *"([^"]+)"/, id)
            device_id = id[1]
        }
        if (dm_array[i] ~ /"device_serial_no"/) {
            match(dm_array[i], /"device_serial_no": *"([^"]+)"/, serial)
            device_serial = serial[1]
            device_map[device_serial] = device_id
        }
    }
}
function print_props() {
    if (props["iSerial"] in device_map && props["iSerial"] in ttyacm_map) {
        printf "{\"device_serial_no\": \"%s\", \"device_id\": \"%s\", \"serial_port\": \"%s\"}\n", 
               props["iSerial"], device_map[props["iSerial"]], ttyacm_map[props["iSerial"]]
    }
    split("", props)
}
/^Bus/ { 
    if (length(props) > 0) {
        print_props()
    }
}
/idVendor/ { props["idVendor"] = $2 }
/idProduct/ { props["idProduct"] = $2 }
/iSerial/ { 
    gsub(/^[ \t]+|[ \t]+$/, "", $3)
    if ($3 != "") props["iSerial"] = $3
}
/iProduct/ { 
    product = substr($0, index($0,$3))
    gsub(/^[ \t]+|[ \t]+$/, "", product)
    gsub(/"/, "\\\"", product)
    if (product != "" && product ~ /STM32/) props["iProduct"] = product
}
END { 
    if (length(props) > 0) {
        print_props()
    }
}
' | jq -s '.'