#!/usr/bin/env bash
# Verify that go.mod and .go-version select the same Go major/minor release.

set -euo pipefail

cd "$(dirname "$0")/../.."

gomod_version="$(awk '/^go [0-9]+\.[0-9]+(\.[0-9]+)?$/ { print $2; exit }' go.mod)"
toolchain_version="$(tr -d '[:space:]' < .go-version)"
version_re='^[0-9]+\.[0-9]+(\.[0-9]+)?$'

if [[ ! "$gomod_version" =~ $version_re ]]; then
    echo "::error::check-go-version.sh: could not parse the go directive in go.mod" >&2
    exit 1
fi
if [[ ! "$toolchain_version" =~ $version_re ]]; then
    echo "::error::check-go-version.sh: .go-version must contain one Go version" >&2
    exit 1
fi

gomod_minor="$(cut -d. -f1,2 <<<"$gomod_version")"
toolchain_minor="$(cut -d. -f1,2 <<<"$toolchain_version")"
if [[ "$gomod_minor" != "$toolchain_minor" ]]; then
    echo "::error::check-go-version.sh: Go version drift: go.mod=$gomod_version, .go-version=$toolchain_version" >&2
    exit 1
fi

echo "check-go-version.sh: OK (Go $toolchain_version; go.mod $gomod_version)"
