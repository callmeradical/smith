# docker/hooks.Dockerfile
# Combined environment for git hooks (Go, Node, Helm, Playwright, Python)

FROM golang:1.25-bookworm AS go-bin
FROM alpine/helm:3.16.4 AS helm-bin
FROM python:3.12-slim-bookworm AS python-bin

FROM mcr.microsoft.com/playwright:v1.50.1-noble

# Copy Go
COPY --from=go-bin /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Copy helm
COPY --from=helm-bin /usr/bin/helm /usr/bin/helm

# Copy make from golang image
COPY --from=go-bin /usr/bin/make /usr/bin/make

# Install zensical via pip from python-bin stage
# We'll just copy the whole python install or use a more surgical approach.
# noble has python3.12 usually.
COPY --from=python-bin /usr/local /usr/local
# Ensure paths are correct for the copied python
ENV PATH="/usr/local/bin:${PATH}"

# Now we should have pip
RUN pip3 install zensical --break-system-packages

WORKDIR /workspace
