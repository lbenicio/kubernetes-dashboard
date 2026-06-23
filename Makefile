ROOT_DIRECTORY := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

include $(ROOT_DIRECTORY)/hack/include/build.mk
include $(ROOT_DIRECTORY)/hack/include/config.mk
include $(ROOT_DIRECTORY)/hack/include/ensure.mk
include $(ROOT_DIRECTORY)/hack/include/kind.mk

include $(API_DIRECTORY)/hack/include/config.mk
include $(WEB_DIRECTORY)/hack/include/config.mk

# List of targets that should be executed before other targets
PRE = --ensure-tools clean

.PHONY: help
help:
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":[^:]*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# ============================ GLOBAL ============================ #
#
# A global list of targets executed for every module, it includes:
# - all modules in 'modules' directory except 'modules/common'
# - all common modules in 'modules/common' directory except 'modules/common/tools'
#
# ================================================================ #

.PHONY: build
build: $(PRE) ## Runs all available checks
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=build

.PHONY: check
check: $(PRE) check-license ## Runs all available checks
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=check

.PHONY: clean
clean: --clean ## Clean up all temporary directories
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=clean

.PHONY: coverage
coverage: $(PRE) ## Runs all available test coverage scripts
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=coverage

.PHONY: fix
fix: $(PRE) fix-license ## Runs all available fix scripts
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=fix

.PHONY: test
test: $(PRE) ## Runs all available test scripts
	@$(MAKE) --no-print-directory -C $(MODULES_DIRECTORY) TARGET=test

# ============================ Local ============================ #

.PHONY: schema
schema: $(PRE)
	@echo "[root] Regenerating schemas"
	@(cd $(API_DIRECTORY) && make --no-print-directory schema)
	@(cd $(WEB_DIRECTORY) && make --no-print-directory schema)
	@echo "[root] Schema regenerated successfully"

.PHONY: check-license
check-license: $(PRE) ## Checks if repo files contain valid license header
	@echo "Running license check"
	@${GOPATH}/bin/license-eye header check

.PHONY: fix-license
fix-license: $(PRE) ## Adds missing license header to repo files
	@echo "Running license check --fix"
	@${GOPATH}/bin/license-eye header fix

.PHONY: tools
tools: $(PRE) ## Installs required tools

# Starts development version of the application.
#
# URL: http://localhost:8080
#
# Note: Make sure that the port 8080 (Web HTTP) is free on your localhost
.PHONY: serve
serve: $(PRE) --ensure-kind-cluster --ensure-metrics-server ## Starts development version of the application on http://localhost:8080
	@KUBECONFIG=$(KIND_CLUSTER_INTERNAL_KUBECONFIG_PATH) \
	SYSTEM_BANNER=$(SYSTEM_BANNER) \
	SYSTEM_BANNER_SEVERITY=$(SYSTEM_BANNER_SEVERITY) \
	SIDECAR_HOST=$(SIDECAR_HOST) \
	docker compose -f $(DOCKER_COMPOSE_DEV_PATH) --project-name=$(PROJECT_NAME) up \
		--build \
		--force-recreate \
		--remove-orphans \
		--no-attach gateway \
		--no-attach scraper \
		--no-attach metrics-server

# Starts production version of the application.
#
# HTTPS: https://localhost:8443
# HTTP: http://localhost:8080
#
# Note: Make sure that the ports 8443 (Gateway HTTPS) and 8080 (Gateway HTTP) are free on your localhost
.PHONY: run
run: $(PRE) --ensure-kind-cluster --ensure-metrics-server ## Starts production version of the application on https://localhost:8443 and https://localhost:8000
	@KUBECONFIG=$(KIND_CLUSTER_INTERNAL_KUBECONFIG_PATH) \
	SYSTEM_BANNER=$(SYSTEM_BANNER) \
	SYSTEM_BANNER_SEVERITY=$(SYSTEM_BANNER_SEVERITY) \
	SIDECAR_HOST=$(SIDECAR_HOST) \
	VERSION="v0.0.0-prod" \
	WEB_BUILDER_ARCH=$(ARCH) \
	docker compose -f $(DOCKER_COMPOSE_PATH) --project-name=$(PROJECT_NAME) up \
		--build \
		--remove-orphans \
		--no-attach gateway \
		--no-attach scraper \
		--no-attach metrics-server

.PHONY: image
image:
ifndef NO_BUILD
		@KUBECONFIG=$(KIND_CLUSTER_INTERNAL_KUBECONFIG_PATH) \
		SYSTEM_BANNER=$(SYSTEM_BANNER) \
		SYSTEM_BANNER_SEVERITY=$(SYSTEM_BANNER_SEVERITY) \
		SIDECAR_HOST=$(SIDECAR_HOST) \
		VERSION="v0.0.0-prod" \
		WEB_BUILDER_ARCH=$(ARCH) \
		docker compose -f $(DOCKER_COMPOSE_PATH) --project-name=$(PROJECT_NAME) build \
		--no-cache
endif

# Prepares and installs local dev version of Kubernetes Dashboard in our dedicated kind cluster.
#
# 1. Build all docker images
# 2. Load images into kind cluster
# 3. Run helm install using loaded dev images
#
# Run "NO_BUILD=true make helm" to skip building images.
#
# URL: https://localhost
#
# Note: Requires kind to set up and run.
# Note #2: Make sure that the port 443 (HTTPS) is free on your localhost.
.PHONY: helm
helm: --ensure-kind-cluster --ensure-kind-ingress-nginx --ensure-helm-dependencies image --kind-load-images ## Install Kubernetes Dashboard dev helm chart in the dev kind cluster
	@helm upgrade \
		--create-namespace \
		--namespace dashboard \
		--install kubernetes-dashboard \
		--set auth.image.repository=dashboard-auth \
		--set auth.image.tag=latest \
		--set api.image.repository=dashboard-api \
		--set api.image.tag=latest \
		--set web.image.repository=dashboard-web \
		--set web.image.tag=latest \
		--set metricsScraper.image.repository=dashboard-scraper \
		--set metricsScraper.image.tag=latest \
		--set metrics-server.enabled=true \
		--set cert-manager.enabled=true \
		--set app.ingress.enabled=true \
		--set app.ingress.ingressClassName=nginx \
		charts/kubernetes-dashboard

# Installs latest version of Kubernetes Dashboard in our dedicated kind cluster.
#
# 1. Run helm install
#
# Run "NO_BUILD=true make helm" to skip building images.
#
# URL: https://localhost
#
# Note: Requires kind to set up and run.
# Note #2: Make sure that the port 443 (HTTPS) is free on your localhost.
.PHONY: helm-release
helm-release: --ensure-kind-cluster --ensure-kind-ingress-nginx --ensure-helm-dependencies ## Install Kubernetes Dashboard helm chart in the dev kind cluster
	@helm upgrade \
		--create-namespace \
		--namespace dashboard \
		--install kubernetes-dashboard \
		--set metrics-server.enabled=true \
		--set app.ingress.enabled=true \
		--set app.ingress.ingressClassName=nginx \
		charts/kubernetes-dashboard

# Tags and pushes locally built Docker images to a container registry.
#
# Version numbers are read from versions.env. Use 'make bump' to update them.
#
# Usage:
#   make bump LEVEL=patch                   # bump all modules
#   make bump LEVEL=minor MODULE=auth       # bump only auth module
#   make release                            # build + tag + push
#   make release DRY_RUN=1                  # preview without executing
#   make release REGISTRY=docker.io/myorg   # custom registry
#   make release VERSION_SUFFIX=-rc1        # add suffix to versions
#
# Variables:
#   REGISTRY       - Container registry prefix (default: docker.io/lbenicio)
#   VERSION_SUFFIX - Optional suffix appended to all versions (e.g., -rc1, -oidc)
#   DRY_RUN        - Set to 1 to preview commands without executing
#   LEVEL          - Bump level for 'make bump': patch | minor | major (default: patch)
#   MODULE         - Module for 'make bump': auth | api | web | scraper | all (default: all)
#   APP            - App to release: auth | api | web | scraper | all (default: all)

REGISTRY ?= docker.io/lbenicio
VERSION_SUFFIX ?=
DRY_RUN ?= 0
LEVEL ?= patch
MODULE ?= all
APP ?= all

# Auto-detect host architecture for the web builder
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# Load versions from the versions tracking file
-include $(ROOT_DIRECTORY)/versions.env

# Fallback defaults if versions.env is missing
AUTH_VERSION ?= 1.4.1
API_VERSION ?= 1.14.1
WEB_VERSION ?= 1.7.1
SCRAPER_VERSION ?= 1.2.2

.PHONY: bump
bump: ## Bump module versions (LEVEL=patch|minor|major, MODULE=auth|api|web|scraper|all)
	@./hack/scripts/bump-version.sh $(LEVEL) $(MODULE)

.PHONY: bump-show
bump-show: ## Show current versions from versions.env
	@echo "Current versions:"
	@grep -E '^(AUTH|API|WEB|SCRAPER)_VERSION=' $(ROOT_DIRECTORY)/versions.env 2>/dev/null || echo "  (no versions.env found — using Makefile defaults)"
	@echo ""
	@echo "  AUTH_VERSION   = $(AUTH_VERSION)"
	@echo "  API_VERSION    = $(API_VERSION)"
	@echo "  WEB_VERSION    = $(WEB_VERSION)"
	@echo "  SCRAPER_VERSION = $(SCRAPER_VERSION)"

.PHONY: release
release: image ## Build, tag and push Docker images (APP=auth|api|web|scraper|all)
	@echo "[release] Registry: $(REGISTRY)"
	@echo "[release] App: $(APP)"
	@for app in auth api web scraper; do \
		if [ "$(APP)" != "all" ] && [ "$(APP)" != "$$app" ]; then continue; fi; \
		case "$$app" in \
			auth)    ver="$(AUTH_VERSION)" ; name="dashboard-auth" ;; \
			api)     ver="$(API_VERSION)"  ; name="dashboard-api" ;; \
			web)     ver="$(WEB_VERSION)"  ; name="dashboard-web" ;; \
			scraper) ver="$(SCRAPER_VERSION)" ; name="dashboard-scraper" ;; \
		esac; \
		echo "[release] $$name: $$ver$(VERSION_SUFFIX)"; \
	done; \
	if [ "$(DRY_RUN)" = "1" ]; then \
		echo "[DRY RUN] Would execute:"; \
		for app in auth api web scraper; do \
			if [ "$(APP)" != "all" ] && [ "$(APP)" != "$$app" ]; then continue; fi; \
			case "$$app" in \
				auth)    ver="$(AUTH_VERSION)" ; name="dashboard-auth" ;; \
				api)     ver="$(API_VERSION)"  ; name="dashboard-api" ;; \
				web)     ver="$(WEB_VERSION)"  ; name="dashboard-web" ;; \
				scraper) ver="$(SCRAPER_VERSION)" ; name="dashboard-scraper" ;; \
			esac; \
			echo "  docker tag $$name:latest $(REGISTRY)/kubernetes-$$name:$$ver$(VERSION_SUFFIX)"; \
			echo "  docker tag $$name:latest $(REGISTRY)/kubernetes-$$name:latest"; \
			echo "  docker push $(REGISTRY)/kubernetes-$$name:$$ver$(VERSION_SUFFIX)"; \
			echo "  docker push $(REGISTRY)/kubernetes-$$name:latest"; \
		done; \
	else \
		echo "[release] Tagging and pushing..."; \
		for app in auth api web scraper; do \
			if [ "$(APP)" != "all" ] && [ "$(APP)" != "$$app" ]; then continue; fi; \
			case "$$app" in \
				auth)    ver="$(AUTH_VERSION)" ; name="dashboard-auth" ;; \
				api)     ver="$(API_VERSION)"  ; name="dashboard-api" ;; \
				web)     ver="$(WEB_VERSION)"  ; name="dashboard-web" ;; \
				scraper) ver="$(SCRAPER_VERSION)" ; name="dashboard-scraper" ;; \
			esac; \
			docker tag $$name:latest $(REGISTRY)/kubernetes-$$name:$$ver$(VERSION_SUFFIX); \
			docker tag $$name:latest $(REGISTRY)/kubernetes-$$name:latest; \
			docker push $(REGISTRY)/kubernetes-$$name:$$ver$(VERSION_SUFFIX); \
			docker push $(REGISTRY)/kubernetes-$$name:latest; \
		done; \
		echo "[release] Done!"; \
	fi

# To serve Dashboard under a different path than root (/) use:
#		--set app.ingress.path=/dashboard \

# To test API mode with helm below options can be used:
#		--set app.mode=api \
#		--set kong.enabled=false \
#		--set api.containers.args={--metrics-provider=none} \

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall helm dev installation of Kubernetes Dashboard
	@helm uninstall -n dashboard kubernetes-dashboard

# ============================ Private ============================ #

.PHONY: --clean
--clean:
	@echo "[root] Cleaning up"
	@rm -rf $(TMP_DIRECTORY)
