# syntax=docker/dockerfile:1.7

ARG ALPINE_TAG=3.21
FROM alpine:${ALPINE_TAG}

ENV HOME=/home/dev \
    SHELL=/bin/sh

# Explicitly reject non-Alpine base images if FROM is changed.
RUN if [ ! -f /etc/os-release ] || ! grep -Eq '^ID="?alpine"?$' /etc/os-release; then \
      echo "ERROR: Dockerfile must use an Alpine base image (FROM alpine:<tag>)." >&2; \
      exit 1; \
    fi

# hadolint ignore=DL3018
RUN apk add --no-cache nodejs npm

# Install Codex CLI from npm latest channel for v1 (no version pin).
# Command: npm install --global @openai/codex@latest
# hadolint ignore=DL3016
RUN npm install --global @openai/codex@latest \
    && command -v codex >/dev/null

RUN addgroup -S -g 1000 dev \
    && adduser -S -D -u 1000 -G dev -h "${HOME}" dev \
    && mkdir -p /workspace "${HOME}/.codex" \
    && chown -R dev:dev /workspace "${HOME}"

WORKDIR /workspace
SHELL ["/bin/sh", "-lc"]
USER dev

CMD ["sh"]
