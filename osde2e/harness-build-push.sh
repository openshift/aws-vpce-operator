#!/usr/bin/env bash
	
HARNESS_IMAGE="quay.io/app-sre/aws-vpce-operator-test-harness"
# Detect the container engine to use, allowing override from the env
CONTAINER_ENGINE=${CONTAINER_ENGINE:-$(command -v podman || command -v docker || true)}
if [[ -z "$CONTAINER_ENGINE" ]]; then
    echo "WARNING: Couldn't find a container engine! Defaulting to docker."
    CONTAINER_ENGINE=docker
fi

${CONTAINER_ENGINE} build --pull osde2e --tag  ${HARNESS_IMAGE}
if [ $? -ne 0 ] ; then
    echo "docker build failed, exiting..."
    exit 1
fi

${CONTAINER_ENGINE} push ${HARNESS_IMAGE}  
