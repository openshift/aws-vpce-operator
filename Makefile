FIPS_ENABLED=true

# needed for FR operators as boilerplate checks commercial app-interface saas file hashes
export SKIP_SAAS_FILE_CHECKS=y

include boilerplate/generated-includes.mk

SHELL := /usr/bin/env bash

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

.PHONY: run
run:
	go run ./main.go

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	oc apply -f ./deploy/crds/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	oc delete --ignore-not-found=$(ignore-not-found) -f ./deploy/crds/

DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
# to ignore vendor directory
GOFLAGS=-mod=mod
.PHONY: osde2e
osde2e:
	CGO_ENABLED=0 go test -v -c ./osde2e/
	mv osde2e.test osde2e/.

.PHONY: harness-build-push
harness-build-push:
	@${DIR}/osde2e/harness-build-push.sh 