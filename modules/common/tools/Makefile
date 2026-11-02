NODE_BINARY := $(shell which node)

YARN_BINARY := $(shell which yarn)

GO_BINARY := $(shell which go)
GO_MAJOR_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f1)
GO_MINOR_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
GO_PATCH_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f3)
MIN_GO_MAJOR_VERSION = 1
MIN_GO_MINOR_VERSION = 27
MIN_GO_PATCH_VERSION = 3

LICENSE_EYE_BINARY := $(shell which license-eye)
AIR_BINARY := $(shell which air)
GOLANGCI_LINT_BINARY := $(shell which golangci-lint)
CLIENT_GEN_BINARY := $(shell which client-gen)
KIND_BINARY := $(shell which kind)

.PHONY: install
install: ensure-node ensure-yarn ensure-go install-license-eye install-air install-golangci-lint install-client-gen install-kind

.PHONY: ensure-node
ensure-node:
ifndef NODE_BINARY
	$(error "Cannot find node binary")
endif
	@node -e 'const [major, minor, patch] = process.versions.node.split(".").map(Number); const supported = (major === 22 && (minor > 22 || (minor === 22 && patch >= 3))) || (major === 24 && minor >= 15) || major === 26; if (!supported) { console.error("Unsupported Node.js " + process.versions.node + "; use ^22.22.3, ^24.15.0, or ^26.0.0."); process.exit(1); } console.log("[tools] node version requirements met");'

.PHONY: ensure-yarn
ensure-yarn:
ifndef YARN_BINARY
	@echo "[tools] yarn required to build front-end"
	@echo "[tools] see how to install yarn at: https://yarnpkg.com/getting-started/install"
else
	@echo "[tools] yarn already exists"
endif

.PHONY: ensure-go
ensure-go:
ifndef GO_BINARY
	$(error "Cannot find go binary")
endif
	@if [ $(GO_MAJOR_VERSION) -gt $(MIN_GO_MAJOR_VERSION) ] || \
		{ [ $(GO_MAJOR_VERSION) -eq $(MIN_GO_MAJOR_VERSION) ] && [ $(GO_MINOR_VERSION) -gt $(MIN_GO_MINOR_VERSION) ]; } || \
		{ [ $(GO_MAJOR_VERSION) -eq $(MIN_GO_MAJOR_VERSION) ] && [ $(GO_MINOR_VERSION) -eq $(MIN_GO_MINOR_VERSION) ] && [ $(GO_PATCH_VERSION) -ge $(MIN_GO_PATCH_VERSION) ]; }; then \
		echo "[tools] go version requirements met"; \
	else \
		echo "[tools] go version older than required v$(MIN_GO_MAJOR_VERSION).$(MIN_GO_MINOR_VERSION).$(MIN_GO_PATCH_VERSION)+"; \
		exit 1; \
	fi

.PHONY: install-license-eye
install-license-eye:
ifndef LICENSE_EYE_BINARY
	@echo "[tools] downloading license-eye..."
	@go install github.com/apache/skywalking-eyes/cmd/license-eye
else
	@echo "[tools] license-eye already exists"
endif

.PHONY: install-air
install-air:
ifndef AIR_BINARY
	@echo "[tools] downloading air..."
	@go install github.com/air-verse/air
else
	@echo "[tools] air already exists"
endif

.PHONY: install-golangci-lint
install-golangci-lint:
ifndef GOLANGCI_LINT_BINARY
	@echo "[tools] downloading golangci-lint..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint
else
	@echo "[tools] golangci-lint already exists"
endif

.PHONY: install-client-gen
install-client-gen:
ifndef CLIENT_GEN_BINARY
	@echo "[tools] downloading client-gen..."
	@go install k8s.io/code-generator/cmd/client-gen
else
	@echo "[tools] client-gen already exists"
endif

.PHONY: install-kind
install-kind:
ifndef KIND_BINARY
	@echo "[tools] downloading kind..."
	@go install sigs.k8s.io/kind
else
	@echo "[tools] kind already exists"
endif
