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

FROM node:22-alpine
RUN apk add --no-cache git bash ca-certificates && npm install -g @openai/codex
COPY --from=builder /out/smith-replica /bin/smith-replica
COPY --from=builder /out/smith /bin/smith
WORKDIR /workspace
RUN mkdir -p /workspace && chown -R node:node /workspace
USER node
ENTRYPOINT ["/bin/smith-replica"]
