#!/usr/bin/env bash
# Installs Kind + verifies all tools are present
set -euo pipefail

echo ">>> Installing Kind..."
KIND_VERSION="v0.22.0"
curl -Lo /tmp/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64"
chmod +x /tmp/kind
sudo mv /tmp/kind /usr/local/bin/kind
echo "[OK] Kind installed: $(kind --version)"

echo ""
echo ">>> Verifying tools..."
for tool in docker kind kubectl helm go; do
  if command -v "$tool" &>/dev/null; then
    echo "[OK] $tool: $(${tool} version --short 2>/dev/null || ${tool} --version 2>/dev/null | head -1)"
  else
    echo "[MISSING] $tool — install it before continuing"
  fi
done