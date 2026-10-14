#!/usr/bin/env bash
set -euo pipefail

plugins=$(helm plugin list)
if awk '$1 == "unittest" { found = 1 } END { exit !found }' <<< "$plugins"; then
  helm plugin uninstall unittest
fi

# v1.2.1. Helm cannot verify Git plugin sources.
helm plugin install --verify=false --version 67056cc631eab9e001e6762a1f601e6c92a71ba7 \
  https://github.com/helm-unittest/helm-unittest.git
