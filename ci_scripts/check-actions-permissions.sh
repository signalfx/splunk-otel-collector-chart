#!/usr/bin/env bash
set -euo pipefail

# Checks that all GitHub Actions workflows declare a top-level permissions block.
# Exits non-zero if any workflow is missing one.

WORKFLOW_DIR=".github/workflows"
errors=0

for f in "${WORKFLOW_DIR}"/*.yaml "${WORKFLOW_DIR}"/*.yml; do
  [ -f "$f" ] || continue

  if ! grep -q '^permissions:' "$f"; then
    echo "FAIL: $(basename "$f") — missing top-level 'permissions:' block"
    errors=$((errors + 1))
  fi
done

if [ "$errors" -gt 0 ]; then
  echo ""
  echo "Found $errors workflow(s) without explicit permissions."
  echo "Add a 'permissions:' block with the minimum scopes needed."
  echo "Read-only workflows: permissions: { contents: read }"
  exit 1
fi

echo "All workflows declare explicit permissions."
