#!/bin/sh
set -eu

: "${SMITH_API_BASE_URL:=http://localhost:8080}"
export SMITH_API_BASE_URL

envsubst '${SMITH_API_BASE_URL}' < /opt/smith/runtime-config.template.js > /tmp/runtime-config.js
