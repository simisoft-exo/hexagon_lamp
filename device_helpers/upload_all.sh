#!/bin/bash

while read port; do
  echo "Uploading to $port"
  arduino-cli upload /home/simion/repos/motors/simplefoc_tuning  -p $port --fqbn STMicroelectronics:stm32:Disco 
done < arduino_ports.txt


