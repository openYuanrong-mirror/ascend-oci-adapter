#!/usr/bin/env bash
# Copyright (c) 2026 Ant Group Corporation.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

version="$1"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="${root}/dist"
name="ascend-oci-adapter_${version}_linux_amd64"
stage="$(mktemp -d)"
trap 'rm -rf "${stage}"' EXIT

mkdir -p "${stage}/${name}/bin" \
  "${stage}/${name}/licenses/ascend-oci-adapter" \
  "${stage}/${name}/licenses/mind-cluster"

(
  cd "${root}/cmd/ascend-oci-adapter"
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.adapterVersion=${version}" \
    -o "${stage}/${name}/bin/ascend-oci-adapter" .
)
install -m 0644 "${root}/LICENSE" \
  "${stage}/${name}/licenses/ascend-oci-adapter/LICENSE"
install -m 0644 "${root}/third_party/mind-cluster/LICENSE" \
  "${stage}/${name}/licenses/mind-cluster/LICENSE"
install -m 0644 "${root}/third_party/mind-cluster/Third_Party_Open_Source_Software_Notice.md" \
  "${stage}/${name}/licenses/mind-cluster/Third_Party_Open_Source_Software_Notice.md"

mkdir -p "${dist}"
tar -C "${stage}" -czf "${dist}/${name}.tar.gz" "${name}"
(cd "${dist}"; sha256sum "${name}.tar.gz" > "${name}.tar.gz.sha256")
