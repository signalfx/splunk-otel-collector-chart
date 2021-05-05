.PHONY: render
render:
	for i in metrics traces logs; do \
		dir=$$i-only; \
		mkdir -p rendered/$$dir; \
		helm template \
			--values rendered/values.yaml \
			--set metricsEnabled=false,tracesEnabled=false,logsEnabled=false,$${i}Enabled=true \
			--output-dir rendered/$$dir default helm-charts/splunk-otel-collector; \
		mv rendered/$$dir/splunk-otel-collector/templates/* rendered/$$dir; \
		rm -rf rendered/$$dir/splunk-otel-collector; \
	done

	mkdir -p rendered/default
	helm template --values rendered/values.yaml --output-dir rendered/default default helm-charts/splunk-otel-collector
	mv rendered/default/splunk-otel-collector/templates/* rendered/default
	rm -rf rendered/default/splunk-otel-collector
