#!/usr/bin/env bash
set -euo pipefail

# Checks that all third-party GitHub Actions are pinned to a full SHA digest.
# Exits non-zero if any unpinned action is found.

SCAN_PATHS=(
  .github/workflows/*.yaml
  .github/workflows/*.yml
  .github/actions/*/action.yaml
  .github/actions/*/action.yml
)
errors=0

while IFS= read -r line; do
  file=$(echo "$line" | cut -d: -f1)
  lineno=$(echo "$line" | cut -d: -f2)
  content=$(echo "$line" | cut -d: -f3-)

  ref=$(echo "$content" | sed -n "s/.*uses:[[:space:]]*['\"]*//" | sed "s/['\"].*$//" | sed 's/[[:space:]]*$//')

  # Skip local actions, empty lines, and shell script false positives
  case "$ref" in
    ./*|*echo*|*grep*|*sed*|"") continue ;;
  esac

  if ! echo "$ref" | grep -qE '@[0-9a-f]{40}'; then
    echo "FAIL: ${file}:${lineno} — ${ref}"
    errors=$((errors + 1))
  fi
done < <(grep -rn 'uses:' ${SCAN_PATHS[@]} 2>/dev/null \
  | grep -v '^\s*#' \
  | grep -v '#.*uses:')

if [ "$errors" -gt 0 ]; then
  echo ""
  echo "Found $errors action(s) not pinned to a commit SHA."
  echo "Pin using: org/action@<sha> # <tag>"
  echo "Resolve with: git ls-remote --tags https://github.com/<org>/<action>.git <tag>"
  exit 1
fi

echo "All actions are pinned to SHA digests."
