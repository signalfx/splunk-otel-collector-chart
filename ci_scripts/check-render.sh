#!/bin/bash
# Renders examples and then checks for untracked files under examples/.
# If make render recreates files that were deleted from git, those files
# appear as untracked and indicate an incorrect deletion in the PR.

set -euo pipefail

make render

untracked=$(git ls-files --others --exclude-standard examples/)
if [ -n "$untracked" ]; then
    echo ""
    echo "ERROR: make render created files that are not tracked by git:"
    echo "$untracked"
    echo ""
    echo "Either commit these files or check if they were incorrectly deleted."
    exit 1
fi
