SHELL = /usr/bin/env bash -o pipefail

REPO              ?= ghcr.io/conduitio/conduit-operator
TAG               := $(shell date +%s)
IMG               ?= $(REPO):$(TAG)
NAME              := conduit-operator

CERT_MANAGER      := https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml

KUSTOMIZE_VERSION ?= v5.5.0
CTRL_GEN_VERSION  ?= v0.17.2
KIND_VERSION      ?= v0.24.0
GOLINT_VERSION    ?= v1.63.4

.EXPORT_ALL_VARIABLES:

KIND_CLUSTER          ?= conduit-sandbox
KIND_CONTEXT          := kind-$(KIND_CLUSTER)

.SHELLFLAGS       := -ec
PATH              := $(CURDIR)/bin:$(PATH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: crds manifests fmt vet build

.PHONY: generate
bin/mockgen:
	GOBIN=$(PWD)/bin go install github.com/golang/mock/mockgen@v1.6.0

generate: bin/mockgen
	go generate ./...

.PHONY: fmt
fmt: bin/gofumpt
	gofumpt -l -w .

bin/gofumpt:
	GOBIN=$(PWD)/bin go install mvdan.cc/gofumpt@latest

.PHONY: vet
vet: lint

.PHONY: test
test:
	go test -race ./... -coverprofile cover.out

.PHONY: vet lint
vet: lint
lint: bin/golangci-lint
	golangci-lint run -v

gomod:
	go mod tidy

bin/golangci-lint:
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLINT_VERSION)

.PHONY: build
build:
	go build -o bin/conduit-operator cmd/operator/main.go

# Rebuild and kustomize custom resource definitions

bin/kustomize:
	GOBIN=$(PWD)/bin go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

bin/controller-gen:
	GOBIN=$(PWD)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CTRL_GEN_VERSION)

# Deployment manager generation is disable due to compatibility
# issues with non-simple yaml types and helm/go templates
.PHONY: manifests
manifests: bin/kustomize generate
	kustomize build config/crd > charts/conduit-operator/templates/crd.yaml
	kustomize build config/rbac > charts/conduit-operator/templates/rbac.yaml
	# kustomize build config/manager > charts/conduit-operator/templates/deployment.yaml
	kustomize build config/certmanager > charts/conduit-operator/templates/certificate.yaml
	bin/kustomize build config/webhook > charts/conduit-operator/templates/webhook.yaml

.PHONY: crds
crds: bin/controller-gen
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
	controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

## Create docker image

.PHONY: docker_build
docker_build:
	DOCKER_BUILDKIT=1 docker build -t $(IMG) .

.PHONY: docker_push
docker_push: docker_build
	DOCKER_BUILDKIT=1 docker push $(IMG)

## Tooling for running the operator in a local kubernetes cluster

bin/kind:
	GOBIN=$(PWD)/bin go install sigs.k8s.io/kind/cmd/kind@$(KIND_VERSION)

.PHONY: kind
kind_cluster: bin/kind
	@kind get clusters | grep $(KIND_CLUSTER) || \
		kind create cluster -n $(KIND_CLUSTER)

.PHONY: kind_setup
kind_setup: kind_cluster
	@kubectl config use-context $(KIND_CONTEXT)
	kubectl get crds certificates.cert-manager.io 2>&1 >/dev/null || \
		kubectl apply -f $(CERT_MANAGER)
	@kubectl wait --timeout=-1s --for=condition=Available deployments --all -n cert-manager
	@kubectl wait --timeout=-1s --for=condition=Ready pods --all -n cert-manager

.PHONY: kind_image_build
kind_image_build: docker_build
	@sleep 2 # this is required to allow for docker deskop on mac to settle with the image
	kind load docker-image $(IMG) -n $(KIND_CLUSTER)

.PHONY: dev_delete
dev_delete:
	kind delete cluster -n $(KIND_CLUSTER)

.PHONY: dev
dev: crds manifests kind_setup kind_image_build helm_upgrade
helm_upgrade:
	helm upgrade $(NAME) \
		--debug \
		--atomic \
		--create-namespace --namespace $(NAME) \
		--install \
		--kube-context $(KIND_CONTEXT) \
		--reuse-values \
		--timeout 2m \
		--version 0.1.0 \
		--wait \
		--set "service.type=NodePort" \
		--set "image.tag=$(TAG)" \
		--set "image.pullPolicy=Never" \
		$(HELM_EXTRA_OPTS) \
		$(CURDIR)/charts/conduit-operator
