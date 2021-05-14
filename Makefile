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
			--set metricsEnabled=false,tracesEnabled=false,logsEnabled=false,$${i}Enabled=true \
			--output-dir "$$dir" default helm-charts/splunk-otel-collector; \
		mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
		rm -rf "$$dir"/splunk-otel-collector; \
	done

	# All telemetry types but no gateway, only agent.
	dir=rendered/manifests/agent-only; \
	mkdir -p "$$dir"; \
	helm template --namespace default --values rendered/values.yaml --output-dir "$$dir" \
		default helm-charts/splunk-otel-collector; \
	mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
	rm -rf "$$dir"/splunk-otel-collector

	# XXX: Disable for now, reenable on otel-logs branch.
	# # All telemetry types but no agent, only gateway.
	# mkdir -p rendered/manifests/gateway-only
	# helm template --namespace default --values rendered/values.yaml --output-dir rendered/manifests/gateway-only \
	# 	--set otelAgent.enabled=false,otelCollector.enabled=true,otelK8sClusterReceiver.enabled=false,fluentd.enabled=false \
	# 	default helm-charts/splunk-otel-collector
	# mv rendered/manifests/gateway-only/splunk-otel-collector/templates/* rendered/manifests/gateway-only
	# rm -rf rendered/manifests/gateway-only/splunk-otel-collector
