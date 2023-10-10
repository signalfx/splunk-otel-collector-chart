##@ General
# The general settings and variables for the project
SHELL := /bin/bash

# TODO: Move CHART_FILE_PATH and VALUES_FILE_PATH here, currently set in multiple places
# The version of the splunk-otel-collector chart
VERSION := $(shell grep "^version:" helm-charts/splunk-otel-collector/Chart.yaml | awk '{print $$2}')

## Location for GO resources
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
CHLOGGEN ?= $(LOCALBIN)/chloggen

CERTMANAGER_VERSION ?= $(shell yq eval ".dependencies[] | select(.name == \"cert-manager\") | .version" helm-charts/splunk-otel-collector/Chart.yaml)

# The help target as provided
.PHONY: help
help: ## Display Makefile help information for all actions
	@awk 'BEGIN {FS = ":.*##"; \
                 printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
          /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
          /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' \
          $(MAKEFILE_LIST)

##@ Initialization
# Tasks for setting up the project environment

.PHONY: install-tools
install-tools: ## Install tools (macOS/Linux)
	LOCALBIN=$(LOCALBIN) GOBIN=$(LOCALBIN) ci_scripts/install-tools.sh || exit 1

##@ Build
# Tasks related to building the Helm chart

.PHONY: repo-update
repo-update: ## Update Helm repositories to latest
	@{ \
	if ! (helm repo list | grep -q open-telemetry) ; then \
		helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts || exit 1; \
	fi ;\
	if ! (helm repo list | grep -q jetstack) ; then \
		helm repo add jetstack https://charts.jetstack.io || exit 1; \
	fi ;\
	helm repo update open-telemetry jetstack || exit 1; \
	}

.PHONY: dep-build
dep-build: ## Build the Helm chart with latest dependencies from the current Helm repositories
	@{ \
	DEP_OK=true ;\
	DIR=helm-charts/splunk-otel-collector ;\
	if ! helm dependencies list $$DIR | grep open-telemetry | grep -q ok ; then DEP_OK=false ; fi ;\
	if ! helm dependencies list $$DIR | grep jetstack | grep -q ok ; then DEP_OK=false ; fi ;\
	if [ "$$DEP_OK" = "false" ] ; then helm dependencies build $$DIR || exit 1; fi ;\
	}

.PHONY: render
render: repo-update dep-build ## Render the Helm chart with the examples as input
	examples/render-examples.sh || exit 1

##@ Changelog
# Tasks related to changelog management

.PHONY: chlog-available
chlog-available: ## Validate the chloggen tool is available
	@if [ -z "$(CHLOGGEN)" ]; then \
		echo "Error: chloggen is not available. Please run 'make install-tools' to install it."; \
		exit 1; \
	fi

# Example Usage:
# 	make chlog-new CHANGE_TYPE=enhancement COMPONENT=agent NOTE="Add X" ISSUES='[42]'
# 	make chlog-new [CHANGE_TYPE=enhancement] [COMPONENT=agent] [NOTE="Add X"] [ISSUES='[42]'] [FILENAME=add-x] [SUBTEXT="Add Y"]
.PHONY: chlog-new
chlog-new: chlog-available ## Creates or updates a YAML file under .chloggen
	ci_scripts/chloggen-new.sh || exit 1

.PHONY: chlog-validate
chlog-validate: chlog-available ## Validates changelog requirements for pull requests
	$(CHLOGGEN) validate || exit 1
	ci_scripts/chloggen-pr-validate.sh || exit 1

.PHONY: chlog-preview
chlog-preview: chlog-validate ## Provide a preview of the generated CHANGELOG.md file for a release
	$(CHLOGGEN) update --dry || exit 1

# Example Usage: make chlog-update
.PHONY: chlog-update
chlog-update: chlog-validate ## Creates an update to CHANGELOG.md for a release entry from content in .chloggen
	$(CHLOGGEN) update --version "[$(VERSION)] - $$(date +'%Y-%m-%d')" || exit 1; \
	ci_scripts/chloggen-update.sh || exit 1

.PHONY: cert-manager
cert-manager: cmctl
	# Consider using cmctl to install the cert-manager once install command is not experimental
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/${CERTMANAGER_VERSION}/cert-manager.yaml
	$(CMCTL) check api --wait=5m

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
CMCTL = $(shell pwd)/bin/cmctl
.PHONY: cmctl
cmctl:
	@{ \
	set -e ;\
	if (`pwd`/bin/cmctl version | grep ${CERTMANAGER_VERSION}) > /dev/null 2>&1 ; then \
		exit 0; \
	fi ;\
	TMP_DIR=$$(mktemp -d) ;\
	curl -L -o $$TMP_DIR/cmctl.tar.gz https://github.com/jetstack/cert-manager/releases/download/$(CERTMANAGER_VERSION)/cmctl-`go env GOOS`-`go env GOARCH`.tar.gz ;\
	tar xzf $$TMP_DIR/cmctl.tar.gz -C $$TMP_DIR ;\
	[ -d bin ] || mkdir bin ;\
	mv $$TMP_DIR/cmctl $(CMCTL) ;\
	rm -rf $$TMP_DIR ;\
	}
