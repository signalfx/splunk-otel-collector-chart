# Default image repository
IMAGE_REPO ?= quay.io/splunko11ytest/dotnet_test

# This .NET application cannot run on arm machines due to .NET runtime limitations
# Was not able to use docker buildx with .NET
.PHONY: push
push:
	docker build --file Dockerfile --no-cache	--tag $(IMAGE_REPO):latest --push .
