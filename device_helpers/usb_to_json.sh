#!/bin/bash

lsusb -v 2>/dev/null | awk '
BEGIN { 
    print "[" 
    first = 1
    split("", props)
}
function print_props() {
    first_prop = 1
    for (p in props) {
        if (!first_prop) printf ","
        first_prop = 0
        printf "\"%s\": \"%s\"", p, props[p]
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
    printf "\"device\": \"%s\",", $4
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
'
