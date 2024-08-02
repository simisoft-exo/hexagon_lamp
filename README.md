# Hexagon lamp notes

Currently there are two sub-dirs

## simplefoc_tuning
Intended as the first draft of the tuning and connection sketch
I'm implementing the communication protocol and state management as well as the FOC tuning with SimpleFOC

## device_helpers
Kind of the "devops" of the device:
- compilation
- upload
- debug and testing scripts
-

## Raspberri Pi tools needed

```bash
sudo apt install openjdk-21-jre-headless
sudo snap install go --classic
sudo snap install jq
sudo apt install stlink-tools
## babashka
bash < <(curl -s https://raw.githubusercontent.com/babashka/babashka/master/install)
## arduino-cli
curl -fsSL https://raw.githubusercontent.com/arduino/arduino-cli/master/install.sh | sh
echo 'export PATH="$PATH:/home/hexagon/repos/hexagon_lamp/device_helpers/bin"' >> ~/.bashrc
arduino-cli config add board_manager.additional_urls https://github.com/stm32duino/BoardManagerFiles/raw/main/package_stmicroelectronics_index.json

``` 
