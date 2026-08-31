
PLATFORM ?= linux/amd64
IMAGE ?= python_test:latest

.PHONY: build
build:
	docker build --file Dockerfile --platform $(PLATFORM) --tag $(IMAGE) .
