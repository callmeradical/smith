# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith-replica ./cmd/smith-replica
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith ./cmd/smith

FROM node:22-alpine3.20
RUN apk add --no-cache git bash ca-certificates curl bzip2 && \
    apk upgrade --no-cache libcrypto3 libssl3 && \
    npm install -g @openai/codex
RUN curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | CONFIGURE=false bash
ENV PATH="/root/.local/bin:${PATH}"
COPY --from=builder /out/smith-replica /bin/smith-replica
COPY --from=builder /out/smith /bin/smith
WORKDIR /workspace
RUN mkdir -p /workspace && chown -R node:node /workspace
USER node
ENTRYPOINT ["/bin/smith-replica"]
