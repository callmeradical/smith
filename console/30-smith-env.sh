#!/bin/sh
set -eu

: "${SMITH_API_BASE_URL:=http://localhost:8080}"
: "${SMITH_OPERATOR_TOKEN:=}"
export SMITH_API_BASE_URL
export SMITH_OPERATOR_TOKEN

envsubst '${SMITH_API_BASE_URL} ${SMITH_OPERATOR_TOKEN}' < /opt/smith/runtime-config.template.js > /tmp/runtime-config.js
