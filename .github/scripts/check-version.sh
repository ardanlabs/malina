#!/usr/bin/env bash
# Verify a release tag against the authoritative Malina SDK version.

set -euo pipefail

cd "$(dirname "$0")/../.."

tag="${1:-${GITHUB_REF:-}}"
tag="${tag#refs/tags/}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "::error::check-version.sh: expected a vX.Y.Z tag, got '${tag:-<empty>}'" >&2
    exit 1
fi

version_file="sdk/malina/version.go"
actual="$(sed -nE 's/^const Version = "([0-9]+\.[0-9]+\.[0-9]+)"$/\1/p' "$version_file")"
if [[ -z "$actual" ]]; then
    echo "::error::check-version.sh: could not parse const Version from $version_file" >&2
    exit 1
fi
if [[ "$actual" != "${tag#v}" ]]; then
    echo "::error::check-version.sh: $tag does not match $version_file Version=$actual" >&2
    exit 1
fi

echo "check-version.sh: OK ($tag matches $version_file)"
