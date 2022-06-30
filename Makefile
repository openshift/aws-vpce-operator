FIPS_ENABLED=true

# TODO: Remove below DEPLOYED_HASH once a production CSV bundle is pushed.
#       This is chicken and egg problem with onboarding a new operator.
DEPLOYED_HASH := 70ed7725f82e6e4a3a0986a128b814cd76debfbf
export DEPLOYED_HASH

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
