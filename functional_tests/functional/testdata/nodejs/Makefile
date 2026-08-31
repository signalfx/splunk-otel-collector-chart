
PLATFORM ?= linux/amd64
IMAGE ?= nodejs_test:latest

.PHONY: build
build:
	docker build --file Dockerfile --platform $(PLATFORM) --tag $(IMAGE) .
