.PHONY: render
render: repo-update dep-build
	bash ./examples/render-examples.sh

.PHONY: repo-update
repo-update:
	@{ \
	if ! (helm repo list | grep -q open-telemetry) ; then \
		helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts ;\
	fi ;\
	if ! (helm repo list | grep -q jetstack) ; then \
		helm repo add jetstack https://charts.jetstack.io ;\
	fi ;\
	helm repo update open-telemetry jetstack ;\
	}

.PHONY: dep-build
dep-build:
	helm dependencies build ./helm-charts/splunk-otel-collector
