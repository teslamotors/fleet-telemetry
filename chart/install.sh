#!/usr/bin/env bash

ENV=prod
NAMESPACE=tesla-fleet
RELEASE_NAME=tesla-fleet
CHART_NAME=.

helm upgrade --install -n "${NAMESPACE}" --create-namespace "${RELEASE_NAME}" \
  -f values.yaml -f "${ENV}.yaml" "${CHART_NAME}"
