# Makefile for cross-compiling to Raspberry Pi

# Variables
DOCKER_IMAGE := arm64v8/golang:1.23
DOCKER_IMAGE_TAR := docker_image.tar
EXECUTABLE_NAME := hexagon
RPI_ADDRESS := hexagon.local
RPI_USER := hexagon
REMOTE_PATH := /home/hexagon/repos/hexagon_lamp/

GO_FILES := $(wildcard *.go)

SUBFOLDER := device_helpers
GO_MAIN := tui-debug.go  # Change this to your main Go file name
GO_MAIN_PATH := $(SUBFOLDER)/$(GO_MAIN)

# Check if we need to use sudo for docker commands
DOCKER_SUDO := $(shell if docker info > /dev/null 2>&1; then echo ""; else echo "sudo"; fi)

# Default target
all: setup compile deploy

# Setup: Install Docker and pull the ARM64 Docker image
setup: install-docker pull-docker-image

# Install Docker (this is a basic example, might need adjustments based on the OS)
install-docker:
	@echo "Installing Docker..."
	@which docker > /dev/null || (sudo apt-get update && sudo apt-get install -y docker.io)
	@sudo systemctl start docker
	@sudo systemctl enable docker

# Pull and save the Docker image
pull-docker-image:
	@echo "Pulling Docker image..."
	docker pull $(DOCKER_IMAGE)
	docker save $(DOCKER_IMAGE) > $(DOCKER_IMAGE_TAR)

# Compile the code inside the Docker container
compile:
	@echo "Compiling $(GO_MAIN) in $(SUBFOLDER)..."
	$(DOCKER_SUDO) docker run --rm -v $(PWD):/go/src/app -w /go/src/app/$(SUBFOLDER) $(DOCKER_IMAGE) \
		go build -o ../$(EXECUTABLE_NAME) ./$(GO_MAIN)
# Deploy the executable to Raspberry Pi
deploy:
	@echo "Deploying to Raspberry Pi..."
	scp $(EXECUTABLE_NAME) $(RPI_USER)@$(RPI_ADDRESS):$(REMOTE_PATH)

# Clean up
clean:
	@echo "Cleaning up..."
	rm -f $(EXECUTABLE_NAME)
	rm -f $(DOCKER_IMAGE_TAR)

.PHONY: all setup install-docker pull-docker-image compile deploy clean
