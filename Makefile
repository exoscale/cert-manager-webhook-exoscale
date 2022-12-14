PROJECT_URL = https://github.com/exoscale/cert-manager-webhook-exoscale
include go.mk/init.mk
include go.mk/public.mk

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

IMAGE_NAME := "exoscale/cert-manager-webhook-exoscale"

OUT := $(shell pwd)/_out

KUBE_VERSION=1.24.2

$(shell mkdir -p "$(OUT)")
export TEST_ASSET_ETCD=_test/kubebuilder/bin/etcd
export TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/bin/kube-apiserver
export TEST_ASSET_KUBECTL=_test/kubebuilder/bin/kubectl

integration-test: _test/kubebuilder
	TEST_ZONE_NAME=$(TEST_ZONE_NAME) go test -v .

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(ARCH) -o kubebuilder-tools.tar.gz
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
	    --name exoscale-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
        deploy/exoscale-webhook > "$(OUT)/rendered-manifest.yaml"

