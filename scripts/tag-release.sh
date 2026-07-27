#!/usr/bin/env bash
# tag-release.sh — cut a version tag at a significant milestone, so you can
# roll back to it or branch from it later.
#
#   Usage:  bash scripts/tag-release.sh vX.Y.Z "short description of the milestone"
#   Example: bash scripts/tag-release.sh v1.1.0 "aqd ID-card flow + entity search + TUI parity"
#
# Rules enforced:
#   - Tag name must be semver-shaped: vMAJOR.MINOR.PATCH
#       MAJOR = breaking API change · MINOR = new backward-compatible features
#       PATCH = bug fixes only
#   - Tags are ANNOTATED (carry message + author + date), never lightweight.
#   - Working tree must be clean and pushed (tag a committed, shared state).
#   - The tag is pushed to origin so every station can see and use it.
set -euo pipefail

tag="${1:-}"
msg="${2:-}"

if [[ -z "$tag" || -z "$msg" ]]; then
  echo "usage: bash scripts/tag-release.sh vX.Y.Z \"milestone description\""
  exit 1
fi
if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "!!  '$tag' is not vMAJOR.MINOR.PATCH (e.g. v1.1.0)"
  exit 1
fi
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "!!  Tag '$tag' already exists. Pick the next number."
  exit 1
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "!!  Working tree not clean — commit or stash before tagging."
  git status --short
  exit 1
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
echo "==> Tagging $branch @ $(git rev-parse --short HEAD) as $tag"
git tag -a "$tag" -m "$msg"
git push origin "$tag"
echo "==> Pushed $tag to origin."
echo ""
echo "To go back to this point later:"
echo "    git switch -c hotfix-from-$tag $tag     # new branch from the tag"
echo "    git checkout $tag                       # inspect (detached HEAD)"
