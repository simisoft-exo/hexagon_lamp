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

lsusb -v 2>/dev/null | awk -v ttyacm_info="$TTYACM_INFO" '
BEGIN { 
    print "["
    first = 1
    split("", props)
    split(ttyacm_info, ttyacm_array, "\n")
    for (i in ttyacm_array) {
        split(ttyacm_array[i], temp, ":")
        ttyacm_map[temp[2]] = temp[1]
    }
}
function print_props() {
    first_prop = 1
    for (p in props) {
        if (!first_prop) printf ","
        first_prop = 0
        printf "\"%s\": \"%s\"", p, props[p]
    }
    if (props["iSerial"] in ttyacm_map) {
        if (!first_prop) printf ","
        printf "\"serialPort\": \"%s\"", ttyacm_map[props["iSerial"]]
    }
    split("", props)
}
/^Bus/ { 
    if (!first) {
        print_props()
        print "}"
    }
    first = 0
    print (NR > 2 ? "," : "") "{"
    printf "\"bus\": \"%s\",", $2
    dev = $4
    sub(":$", "", dev)  # Remove trailing colon from device number
    printf "\"device\": \"%s\",", dev
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
    if (product != "") props["iProduct"] = product
}
END { 
    if (!first) {
        print_props()
        print "}"
    }
    print "]" 
}
' | jq '[.[] | select(.iProduct | contains("STM32"))]'
