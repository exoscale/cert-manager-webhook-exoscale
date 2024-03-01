GO_MK_REF := v1.0.0

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

OUT := ${PWD}/_out

DEPLOY_DIR := $(PWD)/deploy/exoscale-webhook

KUBE_VERSION=1.27.1

$(shell mkdir -p "$(OUT)")
export TEST_ASSET_ETCD=_test/kubebuilder/bin/etcd
export TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/bin/kube-apiserver
export TEST_ASSET_KUBECTL=_test/kubebuilder/bin/kubectl

integration-test: _test/kubebuilder
	TEST_ZONE_NAME=$(TEST_ZONE_NAME) go test -v .

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(GOARCH) -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder/bin _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder

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
