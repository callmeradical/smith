#!/bin/sh
set -eu

: "${SMITH_API_BASE_URL:=/api}"
: "${SMITH_OPERATOR_TOKEN:=}"
: "${SMITH_SHOW_COMING_SOON_PROVIDERS:=false}"
export SMITH_API_BASE_URL
export SMITH_OPERATOR_TOKEN
export SMITH_SHOW_COMING_SOON_PROVIDERS

envsubst '${SMITH_API_BASE_URL} ${SMITH_OPERATOR_TOKEN} ${SMITH_SHOW_COMING_SOON_PROVIDERS}' < /opt/smith/runtime-config.template.js > /tmp/runtime-config.js
