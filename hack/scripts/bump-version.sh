#!/usr/bin/env bash
# Copyright 2017 The Kubernetes Authors.
#
# Bumps semver versions in the versions.env file.
#
# Usage:
#   ./bump-version.sh <level> <module>
#
#   level:  patch | minor | major
#   module: auth | api | web | scraper | all

set -euo pipefail

LEVEL="${1:-patch}"
MODULE="${2:-all}"
VERSION_FILE="$(dirname "$0")/../../versions.env"

bump_semver() {
  local version="$1"
  local level="$2"

  # Strip any suffix like -oidc, -rc1
  local base="${version%%-*}"
  local suffix="${version#$base}"

  IFS='.' read -r major minor patch <<< "$base"

  case "$level" in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
    *)
      echo "Error: unknown bump level '$level'. Use patch, minor, or major."
      exit 1
      ;;
  esac

  echo "${major}.${minor}.${patch}${suffix}"
}

update_version() {
  local key="$1"
  local new_version="$2"

  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "s/^${key}=.*/${key}=${new_version}/" "$VERSION_FILE"
  else
    sed -i "s/^${key}=.*/${key}=${new_version}/" "$VERSION_FILE"
  fi
}

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "Error: $VERSION_FILE not found."
  exit 1
fi

bump_module() {
  local key="$1"
  local current
  current=$(grep "^${key}=" "$VERSION_FILE" | cut -d'=' -f2)
  if [[ -z "$current" ]]; then
    echo "Warning: $key not found in $VERSION_FILE, skipping."
    return
  fi

  local new
  new=$(bump_semver "$current" "$LEVEL")
  update_version "$key" "$new"
  echo "  $key: $current → $new"
}

echo "Bumping versions (level=$LEVEL, module=$MODULE):"

case "$MODULE" in
  all)
    bump_module "AUTH_VERSION"
    bump_module "API_VERSION"
    bump_module "WEB_VERSION"
    bump_module "SCRAPER_VERSION"
    ;;
  auth)   bump_module "AUTH_VERSION" ;;
  api)    bump_module "API_VERSION" ;;
  web)    bump_module "WEB_VERSION" ;;
  scraper) bump_module "SCRAPER_VERSION" ;;
  *)
    echo "Error: unknown module '$MODULE'. Use auth, api, web, scraper, or all."
    exit 1
    ;;
esac

echo "Done. Run 'make release' to build and push with new versions."
