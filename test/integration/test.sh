#!/bin/sh

docker run --net=app_default --name integration-test --rm fleet-telemetry-integration-tests
