PLATFORM ?= linux/amd64
IMAGE ?= dotnet_test:latest

.PHONY: build
build:
	docker build --file Dockerfile --platform $(PLATFORM) --tag $(IMAGE) .
