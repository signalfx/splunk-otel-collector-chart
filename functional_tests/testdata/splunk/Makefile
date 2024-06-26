# Default image repository
IMAGE_REPO_QUAY ?= quay.io/splunko11ytest/splunk
IMAGE_REPO_DOCKER ?= splunk/splunk
IMAGE_VERSION ?= 8.2.0

# Note: The Splunk image does not support ARM architecture at this time.
# Use Skopeo to copy the image between registries, allowing local developers with ARM machines to execute this action.
.PHONY: push
push:
	# Copy the image from Docker Hub to Quay
	skopeo copy --all docker://$(IMAGE_REPO_DOCKER):$(IMAGE_VERSION) docker://$(IMAGE_REPO_QUAY):$(IMAGE_VERSION)
