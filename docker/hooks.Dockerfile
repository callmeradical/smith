# docker/hooks.Dockerfile
# Combined environment for git hooks (Go, Node, Helm, Playwright, Python, Trivy)

# Use a highly-stable, fully-loaded base image to avoid apt-get/overlayfs issues entirely
# Just download Go directly and install pip/helm from this.
FROM public.ecr.aws/docker/library/ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="/usr/local/go/bin:/usr/local/bin:${PATH}"

# Combine all installations into a single layer to avoid layer/overlay limits
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    tar \
    wget \
    make \
    python3 \
    python3-pip \
    python3-setuptools \
    xvfb \
    ca-certificates \
    jq \
    && curl -L https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -o go.tar.gz && tar -C /usr/local -xzf go.tar.gz && rm go.tar.gz \
    && curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 && chmod 700 get_helm.sh && ./get_helm.sh && rm get_helm.sh \
    && curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.56.1 \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g npm@latest \
    && npm install -g playwright@1.50.1 \
    && npx playwright install --with-deps chromium \
    && pip3 install --upgrade pip \
    && pip3 install zensical \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
