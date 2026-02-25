GO_MK_REF := v2.0.4

# make go.mk a dependency for all targets
.EXTRA_PREREQS = go.mk

ifndef MAKE_RESTARTS
# This section will be processed the first time that make reads this file.

# This causes make to re-read the Makefile and all included
# makefiles after go.mk has been cloned.
Makefile:
	@touch Makefile
endif

.PHONY: go.mk
.ONESHELL:
go.mk:
	@if [ ! -d "go.mk" ]; then
		git clone https://github.com/exoscale/go.mk.git
	fi
	@cd go.mk
	@if ! git show-ref --quiet --verify "refs/heads/${GO_MK_REF}"; then
		git fetch
	fi
	@if ! git show-ref --quiet --verify "refs/tags/${GO_MK_REF}"; then
		git fetch --tags
	fi
	git checkout --quiet ${GO_MK_REF}

PROJECT_URL = https://github.com/exoscale/cert-manager-webhook-exoscale
go.mk/init.mk:
include go.mk/init.mk
go.mk/public.mk:
include go.mk/public.mk

GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "exoscale/cert-manager-webhook-exoscale"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

DEPLOY_DIR := $(PWD)/deploy/exoscale-webhook

docker-build:
	docker build \
		-t ${IMAGE_NAME} \
		--build-arg VERSION="${VERSION}" \
		--build-arg VCS_REF="${GIT_REVISION}" \
		--build-arg BUILD_DATE="$(shell date -u +"%Y-%m-%dT%H:%m:%SZ")" \
		.
	docker tag ${IMAGE_NAME}:latest ${IMAGE_NAME}:${VERSION}

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    exoscale-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag="latest" \
        --namespace cert-manager \
        ${DEPLOY_DIR} > "$(OUT)/rendered-manifest.yaml"
	cp "${OUT}/rendered-manifest.yaml" "${DEPLOY_DIR}-kustomize/deploy.yaml"

# FIXME: The environment variables are required by the test helper in cert-manager, but not required to run the tests.
integration-test: setup-envtest
	TEST_ASSET_ETCD=$(GO_BIN_OUTPUT_DIR)/k8s/$(ENVTEST_K8S_VERSION)-$(OS)-$(GOARCH)/etcd \
	TEST_ASSET_KUBE_APISERVER=$(GO_BIN_OUTPUT_DIR)/k8s/$(ENVTEST_K8S_VERSION)-$(OS)-$(GOARCH)/kube-apiserver \
	TEST_ASSET_KUBECTL=$(GO_BIN_OUTPUT_DIR)/k8s/$(ENVTEST_K8S_VERSION)-$(OS)-$(GOARCH)/kubectl \
	$(GO) test -v .

# FIXME: Required to set the environment variables below. Remove when fixed.
ENVTEST_K8S_VERSION=1.35.0

HELM_FILES := $(shell find deploy/exoscale-webhook)

## Tool Binaries

ENVTEST ?= $(GO_BIN_OUTPUT_DIR)/setup-envtest

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(GO_BIN_OUTPUT_DIR)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(GO_BIN_OUTPUT_DIR)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(GO_BIN_OUTPUT_DIR)" go install $${package} ;\
mv "$(GO_BIN_OUTPUT_DIR)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
