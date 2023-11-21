
.PHONY: push
push: build
	docker buildx build --file Dockerfile --platform linux/amd64,linux/arm64 --tag quay.io/splunko11ytest/nodejs_test:latest --push .
