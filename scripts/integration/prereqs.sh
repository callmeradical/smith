#!/usr/bin/env bash
set -euo pipefail

install_k3d() {
  if command -v k3d >/dev/null 2>&1; then
    return
  fi
  if command -v brew >/dev/null 2>&1; then
    brew install k3d
    return
  fi
  if command -v curl >/dev/null 2>&1; then
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.8.3 bash
    return
  else
    echo "k3d missing and no supported installer found" >&2
    exit 1
  fi
}

install_vcluster() {
  if command -v vcluster >/dev/null 2>&1; then
    return
  fi
  if command -v brew >/dev/null 2>&1; then
    brew install loft-sh/tap/vcluster
    return
  fi
  if command -v curl >/dev/null 2>&1; then
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    case "$arch" in
      x86_64) arch="amd64" ;;
      arm64|aarch64) arch="arm64" ;;
    esac
    url="https://github.com/loft-sh/vcluster/releases/latest/download/vcluster-${os}-${arch}"
    curl -fsSL "$url" -o /tmp/vcluster
    chmod +x /tmp/vcluster
    if command -v sudo >/dev/null 2>&1; then
      sudo mv /tmp/vcluster /usr/local/bin/vcluster
    else
      mv /tmp/vcluster /usr/local/bin/vcluster
    fi
    return
  else
    echo "vcluster missing and no supported installer found" >&2
    exit 1
  fi
}

install_k3d
install_vcluster

echo "prerequisites ready"
