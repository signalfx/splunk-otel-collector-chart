.PHONY: render
render:
	rm -rf rendered/manifests
	# Set for one of each telemetry type.
	for i in metrics traces logs; do \
		dir=rendered/manifests/"$$i-only"; \
		mkdir -p "$$dir"; \
		helm template \
			--namespace default \
			--values rendered/values.yaml \
			--set splunkObservability.metricsEnabled=false,splunkObservability.tracesEnabled=false,splunkObservability.logsEnabled=false,splunkObservability.$${i}Enabled=true \
			--output-dir "$$dir" \
			default helm-charts/splunk-otel-collector; \
		mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
		rm -rf "$$dir"/splunk-otel-collector; \
	done

	# Default configuration deployment.
	dir=rendered/manifests/agent-only; \
	mkdir -p "$$dir"; \
	helm template \
		--namespace default \
		--values rendered/values.yaml \
		--output-dir "$$dir" \
		default helm-charts/splunk-otel-collector; \
	mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
	rm -rf "$$dir"/splunk-otel-collector

	# Gateway mode deployment only.
	dir=rendered/manifests/gateway-only; \
	mkdir -p "$$dir"; \
	helm template \
		--namespace default \
		--values rendered/values.yaml \
		--output-dir "$$dir" \
		--set otelAgent.enabled=false,otelCollector.enabled=true,otelK8sClusterReceiver.enabled=false \
		default helm-charts/splunk-otel-collector; \
	mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
	rm -rf "$$dir"/splunk-otel-collector

	# Native OTel logs collection instead of fluentd.
	dir=rendered/manifests/otel-logs; \
	mkdir -p "$$dir"; \
	helm template \
		--namespace default \
		--values rendered/values.yaml \
		--output-dir "$$dir" \
		--set logsEngine=otel \
		default helm-charts/splunk-otel-collector; \
	mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
	rm -rf "$$dir"/splunk-otel-collector
