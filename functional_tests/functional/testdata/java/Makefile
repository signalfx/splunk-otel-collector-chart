PLATFORM ?= linux/amd64
IMAGE ?= java_test:latest

.PHONY: build
build:
	docker build --file Dockerfile --platform $(PLATFORM) --tag $(IMAGE) .
