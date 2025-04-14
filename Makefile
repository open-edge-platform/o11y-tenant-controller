# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

SHELL := bash -eu -o pipefail

PROJECT_NAME                      := observability-tenant-controller

## Labels to add Docker/Helm/Service CI meta-data.
LABEL_REVISION                    = $(shell git rev-parse HEAD)
LABEL_CREATED                     ?= $(shell date -u "+%Y-%m-%dT%H:%M:%SZ")

VERSION                           ?= $(shell cat VERSION | tr -d '[:space:]')
BUILD_DIR                         ?= ./build
VENDOR_DIR                        ?= ./vendor

## CHART_NAME is specified in Chart.yaml
CHART_NAME                        ?= $(PROJECT_NAME)
## CHART_VERSION is specified in Chart.yaml
CHART_VERSION                     ?= $(shell grep "version:" ./deployments/$(PROJECT_NAME)/Chart.yaml  | cut -d ':' -f 2 | tr -d '[:space:]')
## CHART_APP_VERSION is modified on every commit
CHART_APP_VERSION                 ?= $(VERSION)
## CHART_BUILD_DIR is given based on repo structure
CHART_BUILD_DIR                   ?= $(BUILD_DIR)/chart/
## CHART_PATH is given based on repo structure
CHART_PATH                        ?= "./deployments/$(CHART_NAME)"
## CHART_NAMESPACE can be modified here
CHART_NAMESPACE                   ?= orch-platform
## CHART_TEST_NAMESPACE can be modified here
CHART_TEST_NAMESPACE              ?= orch-infra
## CHART_RELEASE can be modified here
CHART_RELEASE                     ?= $(PROJECT_NAME)

REGISTRY                          ?= 080137407410.dkr.ecr.us-west-2.amazonaws.com
REGISTRY_NO_AUTH                  ?= edge-orch
REPOSITORY                        ?= o11y
REPOSITORY_NO_AUTH                := $(REGISTRY)/$(REGISTRY_NO_AUTH)/$(REPOSITORY)
DOCKER_IMAGE_NAME                 ?= $(PROJECT_NAME)
DOCKER_IMAGE_TAG                  ?= $(VERSION)

TEST_JOB_NAME                     ?= observability-tenant-controller-test
DOCKER_FILES_TO_LINT              := $(shell find . -type f -name 'Dockerfile*' -print )

GOCMD         := GOPRIVATE="github.com/open-edge-platform" CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go
GOCMD_TEST    := GOPRIVATE="github.com/open-edge-platform" CGO_ENABLED=1 GOARCH=amd64 GOOS=linux go
GOEXTRAFLAGS  :=-trimpath -gcflags="all=-spectre=all -N -l" -asmflags="all=-spectre=all" -ldflags="all=-s -w -X main.version=$(shell cat ./VERSION) -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn"

.DEFAULT_GOAL := help
.PHONY: build

## CI Mandatory Targets start
dependency-check:
	@# Help: Unsupported target
	@echo '"make $@" is unsupported'

build:
	@# Help: Builds tenant-controller
	@echo "---MAKEFILE BUILD---"
	$(GOCMD) build $(GOEXTRAFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME)/$(PROJECT_NAME).go
	@echo "---END MAKEFILE BUILD---"

lint: lint-go lint-markdown lint-yaml lint-proto lint-json lint-shell lint-helm lint-docker
	@# Help: Runs all linters

test:
	@# Help: Runs tests and creates a coverage report
	@echo "---MAKEFILE TEST---"
	$(GOCMD_TEST) test $$(go list ./... | grep -v /cmd/observability-tenant-controller) --race -coverprofile $(BUILD_DIR)/coverage.out -covermode atomic
	gocover-cobertura < $(BUILD_DIR)/coverage.out > $(BUILD_DIR)/coverage.xml
	@echo "---END MAKEFILE TEST---"

docker-build:
	@# Help: Builds docker image
	@echo "---MAKEFILE DOCKER-BUILD---"
	go mod vendor
	docker rmi $(REPOSITORY_NO_AUTH)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) --force
	docker build -f Dockerfile \
		-t $(REPOSITORY_NO_AUTH)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
		--build-arg http_proxy="$(http_proxy)" --build-arg https_proxy="$(https_proxy)" --build-arg no_proxy="$(no_proxy)" \
		--platform linux/amd64 --no-cache .
	@echo "---END MAKEFILE DOCKER-BUILD---"

helm-build: helm-clean
	@# Help: Builds the helm chart
	@echo "---MAKEFILE HELM-BUILD---"
	yq eval -i '.version = "$(VERSION)"' $(CHART_PATH)/Chart.yaml
	yq eval -i '.appVersion = "$(VERSION)"' $(CHART_PATH)/Chart.yaml
	yq eval -i '.annotations.revision = "$(LABEL_REVISION)"' $(CHART_PATH)/Chart.yaml
	yq eval -i '.annotations.created = "$(LABEL_CREATED)"' $(CHART_PATH)/Chart.yaml
	helm package \
		--app-version=$(CHART_APP_VERSION) \
		--debug \
		--dependency-update \
		--destination $(CHART_BUILD_DIR) \
		$(CHART_PATH)

	@echo "---END MAKEFILE HELM-BUILD---"

docker-push:
	@# Help: Pushes the docker image
	@echo "---MAKEFILE DOCKER-PUSH---"
	aws ecr create-repository --region us-west-2 --repository-name $(REGISTRY_NO_AUTH)/$(REPOSITORY)/$(DOCKER_IMAGE_NAME) || true
	docker push $(REPOSITORY_NO_AUTH)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
	@echo "---END MAKEFILE DOCKER-PUSH---"

helm-push:
	@# Help: Pushes the helm chart
	@echo "---MAKEFILE HELM-PUSH---"
	aws ecr create-repository --region us-west-2 --repository-name $(REGISTRY_NO_AUTH)/$(REPOSITORY)/charts/$(CHART_NAME) || true
	helm push $(CHART_BUILD_DIR)$(CHART_NAME)*.tgz oci://$(REPOSITORY_NO_AUTH)/charts
	@echo "---END MAKEFILE HELM-PUSH---"

docker-list: ## Print name of docker container image
	@echo "images:"
	@echo "  $(DOCKER_IMAGE_NAME):"
	@echo "    name: '$(REPOSITORY_NO_AUTH)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)'"
	@echo "    version: '$(DOCKER_IMAGE_TAG)'"
	@echo "    gitTagPrefix: 'v'"
	@echo "    buildTarget: 'docker-build'"

helm-list: ## List helm charts, tag format, and versions in YAML format
	@echo "charts:" ;\
  echo "  $(CHART_NAME):" ;\
  echo -n "    "; grep "^version" "${CHART_PATH}/Chart.yaml"  ;\
  echo "    gitTagPrefix: 'v'" ;\
  echo "    outDir: '${CHART_BUILD_DIR}'" ;\

## CI Mandatory Targets end

## Helper Targets start
all: clean vendor build lint test
	@# Help: Runs clean, vendor, build, lint, test targets

clean:
	@# Help: Deletes build and vendor directories
	@echo "---MAKEFILE CLEAN---"
	rm -rf $(BUILD_DIR)
	rm -rf $(VENDOR_DIR)
	@echo "---END MAKEFILE CLEAN---"

helm-clean:
	@# Help: Cleans the build directory of the helm chart
	@echo "---MAKEFILE HELM-CLEAN---"
	rm -rf $(CHART_BUILD_DIR)
	@echo "---END MAKEFILE HELM-CLEAN---"

vendor:
	@# Help: Runs go mod vendor command
	@echo "---MAKEFILE VENDOR---"
	$(GOCMD) mod vendor
	@echo "---END MAKEFILE VENDOR---"

lint-go:
	@# Help: Runs linters for golang source code files
	@echo "---MAKEFILE LINT-GO---"
	golangci-lint -v run
	@echo "---END MAKEFILE LINT-GO---"

lint-markdown:
	@# Help: Runs linter for markdown files
	@echo "---MAKEFILE LINT-MARKDOWN---"
	markdownlint-cli2 '**/*.md' "!.github" "!vendor" "!**/ci/*"
	@echo "---END MAKEFILE LINT-MARKDOWN---"

lint-yaml:
	@# Help: Runs linter for for yaml files
	@echo "---MAKEFILE LINT-YAML---"
	yamllint -v
	yamllint -f parsable -c yamllint-conf.yaml .
	@echo "---END MAKEFILE LINT-YAML---"

lint-proto:
	@# Help: Runs linter for for proto files
	@echo "---MAKEFILE LINT-PROTO---"
	protolint version
	protolint lint -reporter unix api/
	@echo "---END MAKEFILE LINT-PROTO---"

lint-json:
	@# Help: Runs linter for json files
	@echo "---MAKEFILE LINT-JSON---"
	./scripts/lintJsons.sh
	@echo "---END MAKEFILE LINT-JSON---"

lint-shell:
	@# Help: Runs linter for shell scripts
	@echo "---MAKEFILE LINT-SHELL---"
	shellcheck --version
	shellcheck ***/*.sh
	@echo "---END MAKEFILE LINT-SHELL---"

lint-helm:
	@# Help: Runs linter for helm chart
	@echo "---MAKEFILE LINT-HELM---"
	helm version
	helm lint --strict $(CHART_PATH) --values $(CHART_PATH)/values.yaml
	@echo "---END MAKEFILE LINT-HELM---"

lint-docker:
	@# Help: Runs linter for docker files
	@echo "---MAKEFILE LINT-DOCKER---"
	hadolint --version
	hadolint $(DOCKER_FILES_TO_LINT)
	@echo "---END MAKEFILE LINT-DOCKER---"

lint-license:
	@# Help: Runs license check
	@echo "---MAKEFILE LINT-LICENSE---"
	reuse --version
	reuse --root . lint
	@echo "---END MAKEFILE LINT-LICENSE---"

kind-all: helm-clean docker-build kind-load helm-build
	@# Help: Builds docker image and loads it into the kind cluster and builds the helm chart

kind-load:
	@# Help: Loads docker image into the kind cluster
	@echo "---MAKEFILE KIND-LOAD---"
	kind load docker-image $(DOCKER_REGISTRY)$(DOCKER_REPOSITORY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
	@echo "---END MAKEFILE KIND-LOAD---"

proto:
	@# Help: Regenerates proto-based code
	@echo "---MAKEFILE PROTO---"
    # Requires installed: protoc, protoc-gen-go and protoc-gen-go-grpc
    # See: https://grpc.io/docs/languages/go/quickstart/
	protoc api/*.proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --proto_path=.
	@echo "---END-MAKEFILE PROTO---"

install-tools:
	@# Help: Installs tools required for the project
	# Requires installed: asdf
	@echo "---MAKEFILE INSTALL-TOOLS---"
	./scripts/installTools.sh .tool-versions
	@echo "---END MAKEFILE INSTALL-TOOLS---"
## Helper Targets end

list: help
	@# Help: Displays make targets

help:
	@# Help: Displays make targets
	@printf "%-35s %s\n" "Target" "Description"
	@printf "%-35s %s\n" "------" "-----------"
	@grep -E '^[a-zA-Z0-9_%-]+:|^[[:space:]]+@# Help:' Makefile | \
	awk '\
		/^[a-zA-Z0-9_%-]+:/ { \
			target = $$1; \
			sub(":", "", target); \
		} \
		/^[[:space:]]+@# Help:/ { \
			if (target != "") { \
				help_line = $$0; \
				sub("^[[:space:]]+@# Help: ", "", help_line); \
				printf "%-35s %s\n", target, help_line; \
				target = ""; \
			} \
		}' | sort -k1,1
