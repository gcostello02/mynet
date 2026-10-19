#!/usr/bin/env bash
# Build a clean mynet candidate with embedded VCS metadata (does not touch bin/mynet).
set -euo pipefail
REPO="${MYNET_REPO:-$(cd "$(dirname "$0")/.." && pwd)}"
REV="${MYNET_BUILD_REV:-$(git -C "$REPO" rev-parse HEAD)}"
OUT="${MYNET_CANDIDATE_OUT:-$REPO/bin/mynet-v030-candidate}"
WORKDIR="${MYNET_BUILD_WORKTREE:-}"

if [[ -n "$WORKDIR" ]]; then
  rm -rf "$WORKDIR"
  git -C "$REPO" worktree add --detach "$WORKDIR" "$REV"
  BUILD_DIR="$WORKDIR"
else
  BUILD_DIR="$REPO"
  if [[ -n "$(git -C "$REPO" status --porcelain)" ]]; then
    echo "error: working tree dirty; commit or set MYNET_BUILD_WORKTREE for isolated build" >&2
    exit 1
  fi
fi

mkdir -p "$(dirname "$OUT")"
BUILD_GOOS="${GOOS:-$(go env GOOS)}"
BUILD_GOARCH="${GOARCH:-$(go env GOARCH)}"
BUILD_CGO="${CGO_ENABLED:-0}"
(
  cd "$BUILD_DIR"
  export GOOS="$BUILD_GOOS" GOARCH="$BUILD_GOARCH" CGO_ENABLED="$BUILD_CGO"
  go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/mynet
)

rev="$(go version -m "$OUT" | awk -F= '/vcs.revision=/ {print $2}')"
mod="$(go version -m "$OUT" | awk -F= '/vcs.modified=/ {print $2}')"
echo "built $OUT GOOS=$BUILD_GOOS GOARCH=$BUILD_GOARCH vcs.revision=${rev} vcs.modified=${mod}"
[[ "$mod" == "false" ]] || { echo "vcs.modified must be false" >&2; exit 1; }
[[ "$rev" == "$REV" ]] || { echo "vcs.revision $rev != expected $REV" >&2; exit 1; }

if [[ -n "$WORKDIR" ]]; then
  git -C "$REPO" worktree remove --force "$WORKDIR"
fi
