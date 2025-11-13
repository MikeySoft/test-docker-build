#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${VERSION:-}" ]]; then
  echo "${VERSION#v}"
  exit 0
fi

if git describe --tags --match "v[0-9]*" --abbrev=0 >/dev/null 2>&1; then
  tag="$(git describe --tags --match "v[0-9]*" --abbrev=0)"
  echo "${tag#v}"
  exit 0
fi

sha="$(git rev-parse --short HEAD)"
echo "0.0.0-dev+${sha}"

