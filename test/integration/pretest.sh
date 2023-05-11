#!/bin/sh

set -e

if [ -n "${IT_TARGET}" ]; then
  IT_TARGET_PKGS=$(go list ./... | grep test/integration/${IT_TARGET} )
else
  IT_TARGET_PKGS="./..."
fi

docker build -t fleet-telemetry-integration-tests --build-arg it_target="${IT_TARGET_PKGS}" -f test/integration/Dockerfile .
