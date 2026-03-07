# syntax=docker/dockerfile:1.7

FROM golang:1.22.12-bookworm AS builder
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
    go build -trimpath -ldflags "-s -w -buildid=" -o /out/smith-core ./cmd/smith-core

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/smith-core /bin/smith-core
EXPOSE 8081
ENTRYPOINT ["/bin/smith-core"]
