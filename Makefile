.PHONY: render
render:
	# Set for one of each telemetry type.
	for i in metrics traces logs; do \
		dir=rendered/"$$i-only"; \
		rm -rf "$$dir"; \
		mkdir -p "$$dir"; \
		helm template \
			--values rendered/values.yaml \
			--set metricsEnabled=false,tracesEnabled=false,logsEnabled=false,$${i}Enabled=true \
			--output-dir "$$dir" default helm-charts/splunk-otel-collector; \
		mv "$$dir"/splunk-otel-collector/templates/* "$$dir"; \
		rm -rf "$$dir"/splunk-otel-collector; \
	done

	# All telemetry types but no gateway, only agent.
	rm -rf rendered/agent-only
	mkdir -p rendered/agent-only
	helm template --values rendered/values.yaml --output-dir rendered/agent-only \
		default helm-charts/splunk-otel-collector
	mv rendered/agent-only/splunk-otel-collector/templates/* rendered/agent-only
	rm -rf rendered/agent-only/splunk-otel-collector

	# XXX: Disable for now, reenable on otel-logs branch.
	# # All telemetry types but no agent, only gateway.
	# rm -rf rendered/gateway-only
	# mkdir -p rendered/gateway-only
	# helm template --values rendered/values.yaml --output-dir rendered/gateway-only \
	# 	--set otelAgent.enabled=false,otelCollector.enabled=true,otelK8sClusterReceiver.enabled=false,fluentd.enabled=false \
	# 	default helm-charts/splunk-otel-collector
	# mv rendered/gateway-only/splunk-otel-collector/templates/* rendered/gateway-only
	# rm -rf rendered/gateway-only/splunk-otel-collector
