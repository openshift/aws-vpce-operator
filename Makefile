FIPS_ENABLED=true

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
