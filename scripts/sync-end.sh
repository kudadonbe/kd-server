#!/usr/bin/env bash
# sync-end.sh — run this LAST before you leave any station.
# Makes sure the remote has everything, so the OTHER station can pick up
# exactly where you left off.
#
#   Usage:  bash scripts/sync-end.sh
#
# It will NOT commit for you (commits happen after you've tested). It only
# warns about uncommitted work and pushes commits that are already made.
set -euo pipefail

branch="$(git rev-parse --abbrev-ref HEAD)"
echo "==> Branch: $branch"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "!!  Uncommitted changes still present — these will NOT reach the other station:"
  git status --short
  echo "    Commit them (after testing) or stash, then re-run this script."
fi

# Fetch first so we push onto an up-to-date remote.
git fetch --quiet origin || true

upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
if [[ -z "$upstream" ]]; then
  echo "==> '$branch' has no upstream yet. Pushing and setting it…"
  git push -u origin "$branch"
  exit 0
fi

ahead="$(git rev-list --count "$upstream"..HEAD)"
if [[ "$ahead" -gt 0 ]]; then
  echo "==> Pushing $ahead commit(s) to $upstream…"
  git push
  echo "==> Remote is now current."
else
  echo "==> Nothing to push — remote already has all commits."
fi
