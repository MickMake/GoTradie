#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO=${1:-.}
cd "$REPO"

git apply --check "$SCRIPT_DIR/changes.patch"
git apply "$SCRIPT_DIR/changes.patch"

echo "Applied GoTradie expense import patch. Review with: git status && git diff"
