#!/usr/bin/env bash
# sync-start.sh — run this FIRST every time you sit down at any station.
# Pulls the latest from the remote so you never start work on stale code.
#
#   Usage:  bash scripts/sync-start.sh
#
# It refuses to touch your work if you have uncommitted changes — it only
# fast-forwards, never merges or rebases silently.
set -euo pipefail

branch="$(git rev-parse --abbrev-ref HEAD)"
echo "==> Fetching all remotes (pruning deleted branches)…"
git fetch --all --prune

echo "==> Current branch: $branch"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "!!  You have uncommitted changes. Commit or stash them before syncing."
  git status --short
  exit 1
fi

upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
if [[ -z "$upstream" ]]; then
  echo "!!  '$branch' has no upstream. Set one with:  git push -u origin $branch"
  exit 1
fi

behind="$(git rev-list --count HEAD.."$upstream")"
ahead="$(git rev-list --count "$upstream"..HEAD)"

if [[ "$behind" -gt 0 && "$ahead" -gt 0 ]]; then
  echo "!!  Diverged: $ahead local / $behind remote commits. Merge or rebase manually."
  exit 1
elif [[ "$behind" -gt 0 ]]; then
  echo "==> Fast-forwarding $branch by $behind commit(s)…"
  git merge --ff-only "$upstream"
  echo "==> Up to date."
else
  echo "==> Already up to date${ahead:+ ($ahead local commit(s) not yet pushed)}."
fi

# Keep local main current too, even while working on a feature branch.
if [[ "$branch" != "main" ]] && git show-ref --verify --quiet refs/remotes/origin/main; then
  git fetch origin main:main 2>/dev/null && echo "==> Local 'main' refreshed from origin." || true
fi
