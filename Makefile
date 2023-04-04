.PHONY: cert-manager
cert-manager:
	bash ./ci_scripts/setup_cert_manager.sh

.PHONY: render
render:
	bash ./examples/render-examples.sh
