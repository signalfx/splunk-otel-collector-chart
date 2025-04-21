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

.PHONY: dep-update
dep-update: ## Fetch Helm chart dependency repositories, build the Helm chart with the dependencies specified in the Chart.yaml
	@{ \
	DIR=helm-charts/splunk-otel-collector ;\
	LOCK_FILE=$$DIR/Chart.lock ;\
	if [ -f "$$LOCK_FILE" ] ; then \
		echo "Removing existing Chart.lock file..."; \
		rm -f "$$LOCK_FILE" || exit 1; \
	fi ;\
	if ! (helm repo list | grep -q open-telemetry) ; then \
		helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts || exit 1; \
	fi ;\
	if ! (helm repo list | grep -q jetstack) ; then \
		helm repo add jetstack https://charts.jetstack.io || exit 1; \
	fi ;\
	helm repo update open-telemetry jetstack || exit 1; \
	DEP_OK=true ;\
	if ! helm dependencies list $$DIR | grep open-telemetry | grep -q ok ; then DEP_OK=false ; fi ;\
	if ! helm dependencies list $$DIR | grep jetstack | grep -q ok ; then DEP_OK=false ; fi ;\
	if [ "$$DEP_OK" = "false" ] ; then helm dependencies update $$DIR || exit 1; fi ;\
	if [ -f "$$LOCK_FILE" ] ; then \
		echo "Removing Chart.lock file post-update..."; \
		rm -f "$$LOCK_FILE" || exit 1; \
	fi ;\
	}

# Example Usage:
#		make render
#		make render VALUES="extra-values.yaml"
#		make render VALUES="values1.yaml values2.yaml"
.PHONY: render
render: dep-update ## Render the Helm chart with the examples as input. Users can also provide value overrides.
	@ci_scripts/render-examples.sh $(VALUES) || exit 1

##@ Test
# Tasks related to testing the Helm chart

.PHONY: lint
lint: ## Lint the Helm chart with ct
	@echo "Linting Helm chart..."
	ct lint --config=ct.yaml || exit 1

.PHONY: pre-commit
pre-commit: render ## Test the Helm chart with pre-commit
	@echo "Checking the Helm chart with pre-commit..."
	pre-commit run --all-files || exit 1

.PHONY: unittest
unittest: ## Run unittests on the Helm chart
	@echo "Running unit tests on helm chart..."
	cd helm-charts/splunk-otel-collector && helm unittest --strict -f "../../test/unittests/*.yaml" . || exit 1

# Example Usage:
#   make functionaltest
#   make functionaltest SKIP_SETUP=true SKIP_TEARDOWN=true SKIP_TESTS=true TEARDOWN_BEFORE_SETUP=true SUITE="functional" UPDATE_EXPECTED_RESULTS=true KUBE_TEST_ENV="kind" KUBECONFIG="/path/to/kubeconfig"
.PHONY: functionaltest
functionaltest: ## Run functional tests for this Helm chart with optional tags and environment variables
	@echo "Running functional tests for this helm chart..."
	cd functional_tests && go test -v -timeout 20m ./$(SUITE)/... || exit 1

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
chlog-update: ## Creates an update to CHANGELOG.md for a release entry from content in .chloggen
	$(CHLOGGEN) update --version "[$(VERSION)] - $$(date +'%Y-%m-%d')" || exit 1; \
	ci_scripts/chloggen-update.sh || exit 1
	make chlog-validate || exit 1

# Example Usage:
#		make chlog-release-notes
#		make chlog-release-notes OUTPUT=file
.PHONY: chlog-release-notes
chlog-release-notes: ## Prints out the current release notes to stdout or RELEASE.md
	@ESCAPED_VER=$$(echo $(VERSION) | sed 's/\./\\./g'); \
	VER_PATTERN="\[$${ESCAPED_VER}\]"; \
	echo "Extracting release notes for version $(VERSION)"; \
	if [ "$(OUTPUT)" = "file" ]; then \
		awk "\$$0 ~ /$$VER_PATTERN/{flag=1; next} /^## \[/{flag=0} flag && NF" CHANGELOG.md > helm-charts/splunk-otel-collector/RELEASE.md; \
		echo "Release notes written to RELEASE.md"; \
	else \
		awk "\$$0 ~ /$$VER_PATTERN/{flag=1; next} /^## \[/{flag=0} flag && NF" CHANGELOG.md; \
	fi

##@ Cert Manager
# Tasks related to deploying and managing Cert Manager

.PHONY: cert-manager
cert-manager: cmctl ## Installs cert-manager in the current Kubernetes cluster and verifies API access with cmctl
	# Consider using cmctl to install the cert-manager once install command is not experimental
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/${CERTMANAGER_VERSION}/cert-manager.yaml
	$(CMCTL) check api --wait=5m

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
CMCTL = $(shell pwd)/bin/cmctl
.PHONY: cmctl
cmctl: ## Downloads and installs cmctl, the CLI for cert-manager, to your local system
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

##@ CI Scripts
# Tasks related to continous integration

# Example Usage:
#   make update-docker-image FILE_PATH=./path/to/values.yaml QUERY_STRING='.images.splunk' FILTER='v1.0'
.PHONY: update-docker-image
update-docker-image: ## Updates the Docker image tag in a YAML file to the latest version
	@if [ -z "$(FILE_PATH)" ] || [ -z "$(QUERY_STRING)" ]; then \
		echo "Error: FILE_PATH and QUERY_STRING are mandatory."; \
		echo "Usage: make update-docker-image FILE_PATH=path/to/file.yaml QUERY_STRING='yq.query' FILTER='v1.0' [DEBUG=--debug]"; \
		exit 1; \
	fi
	ci_scripts/update-docker-image.sh "$(FILE_PATH)" "$(QUERY_STRING)" "$(FILTER)" $(DEBUG)

# Example Usage:
#   make update-chart-dep CHART_PATH=./helm-charts/splunk-otel-collector/Chart.yaml SUBCHART_NAME='opentelemetry-operator'
.PHONY: update-chart-dep
update-chart-dep: dep-update ## Updates the dependency version in the Chart.yaml file to the latest version
	@if [ -z "$(CHART_PATH)" ] || [ -z "$(SUBCHART_NAME)" ]; then \
		echo "Error: CHART_PATH and SUBCHART_NAME are mandatory."; \
		echo "Usage: make update-docker-image FILE_PATH=path/to/file.yaml QUERY_STRING='yq.query' [DEBUG=--debug]"; \
		exit 1; \
	fi
	ci_scripts/update-chart-dependency.sh $(CHART_PATH) $(SUBCHART_NAME) $(DEBUG_MODE)

# Usage Examples:
#   make prepare-release
#       - Prepares for a new release using the current chart and app version in Chart.yaml.
#         Automatically increments the chart version.
#         Commits and pushes changes to a new branch unless CREATE_BRANCH is set to "false".
#   make prepare-release CHART_VERSION=1.2.3 APP_VERSION=1.2.0 CREATE_BRANCH=false
#       - Prepares for a new release with the chart version explicitly set to 1.2.3 and the app version to 1.2.0.
#         Changes remain local if CREATE_BRANCH is set to "false", suitable for workflows that do not immediately push to remote.
.PHONY: prepare-release
prepare-release: ## Prepares for a new release of the helm chart. Optionally specify CHART_VERSION and APP_VERSION.
	ci_scripts/prepare-release.sh CREATE_BRANCH=${CREATE_BRANCH} CHART_VERSION=${CHART_VERSION} APP_VERSION=${APP_VERSION}

.PHONY: tidy-all
tidy-all:
	@for dir in $$(find . -type f -name "go.mod" -exec dirname {} \; | sort | egrep '^./'); do \
		echo "Running go mod tidy in $$dir"; \
		(cd "$$dir" && rm -f go.sum && go mod tidy); \
	done

.PHONY: update-operator-crds
update-operator-crds: ## Update CRDs in the opentelemetry-operator-crds subchart
	ci_scripts/update-crds.sh
