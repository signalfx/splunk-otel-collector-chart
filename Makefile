.PHONY: render
render:
	bash ./examples/render-examples.sh

.PHONY: repo-update
repo-update:
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	helm repo add jetstack https://charts.jetstack.io
	helm repo update

.PHONY: dep-build
dep-build:
	helm dependencies build ./helm-charts/splunk-otel-collector
