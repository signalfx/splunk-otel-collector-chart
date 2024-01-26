
.PHONY: push
push:
	docker buildx build --file Dockerfile --platform linux/amd64,linux/arm64 --tag quay.io/splunko11ytest/java_test:latest --push .
