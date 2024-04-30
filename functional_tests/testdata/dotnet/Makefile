# Default image repository
IMAGE_REPO ?= quay.io/splunko11ytest/dotnet_test

# This .NET application cannot currently run on arm machines due to .NET runtime environment limitations
# Was not able to use docker buildx with .NET
.PHONY: push
push:
	docker build --file Dockerfile --no-cache	--tag $(IMAGE_REPO):latest --push .
	# TODO: Remove this if not needed or used
	# docker buildx build --file Dockerfile --platform linux/amd64,linux/arm64 --no-cache=true --tag $(IMAGE_REPO):latest --push .
