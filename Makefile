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
	@sudo usermod -aG docker $(USER)
	@echo "You may need to log out and log back in for Docker permissions to take effect."

# Set up Docker buildx
setup-buildx:
	@echo "Setting up Docker buildx..."
	$(DOCKER_SUDO) docker buildx create --name arm64builder --use || true
	$(DOCKER_SUDO) docker buildx inspect arm64builder --bootstrap

# Compile the code using Docker buildx
compile:
	@echo "Cross-compiling $(GO_MAIN) in $(SUBFOLDER) for ARM64..."
	$(DOCKER_SUDO) docker buildx build --platform linux/arm64 \
		--build-arg SUBFOLDER=$(SUBFOLDER) \
		--build-arg GO_MAIN=$(GO_MAIN) \
		--build-arg EXECUTABLE_NAME=$(EXECUTABLE_NAME) \
		--output type=local,dest=. \
		-f Dockerfile.cross .

deploy:
	@echo "Deploying to Raspberry Pi..."
	scp $(EXECUTABLE_NAME) $(RPI_USER)@$(RPI_ADDRESS):$(REMOTE_PATH)

# Clean up
clean:
	@echo "Cleaning up..."
	rm -f $(EXECUTABLE_NAME)
	rm -f $(DOCKER_IMAGE_TAR)

.PHONY: all setup install-docker pull-docker-image compile deploy clean
