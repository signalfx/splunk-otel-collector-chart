.PHONY: render
render: repo-update dep-build
	bash ./examples/render-examples.sh

.PHONY: repo-update
repo-update:
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	helm repo add jetstack https://charts.jetstack.io
	helm repo update open-telemetry jetstack

.PHONY: dep-build
dep-build:
	helm dependencies build ./helm-charts/splunk-otel-collector

.PHONY: demo-update
demo-update:
	# Convert this into a method or script if other demos are added.
	curl -L wget https://raw.githubusercontent.com/spring-petclinic/spring-petclinic-microservices/master/docker-compose.yml >> examples/enable-operator-and-auto-instrumentation/spring-petclinic/docker-compose.yaml
	kompose convert --file examples/enable-operator-and-auto-instrumentation/spring-petclinic/docker-compose.yaml --out examples/enable-operator-and-auto-instrumentation/spring-petclinic/02_install_resources.yaml
	rm -rf examples/enable-operator-and-auto-instrumentation/spring-petclinic/docker-compose.yaml
