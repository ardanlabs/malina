Read `sdk/malina/version.go` and use its `Version` constant as the release
version. Review every change since the preceding tag, then write factual
Markdown release notes. Do not infer support merely from GoReleaser targets.

Copy the final Markdown to the clipboard and use this structure:

# Release Notes - vX.Y.Z

**Release Date:** <Month DD, YYYY>

## Overview

<Concise summary and intended users.>

## Native Runtime Compatibility

- **stable-diffusion.cpp version/ABI:** <exact pinned version and ABI changes>
- **Platforms and architectures:** <tested native combinations; distinguish
  downloadable CLI targets from native inference support>
- **Backends:** <CPU, Metal, CUDA, Vulkan, or other tested support>
- **Models:** <supported model families and relevant compatibility changes>

State explicitly that release archives do not bundle native libraries or
models and that users install libraries with `malina libs pull`.

## SDK and API

<SDK packages, exported API, behavior, and compatibility changes, or None.>

## CLI and Server

<Commands, flags, configuration, endpoints, and server behavior, or None.>

## Browser UI

<BUI features, fixes, build/runtime requirements, or None.>

## Fixes and Improvements

<Grouped, user-visible fixes, performance work, dependencies, and docs.>

## Breaking Changes

<Exact breakages and affected users, or None.>

## Upgrade Guide

<Required migration steps, native library refresh instructions, configuration
changes, and recommended verification. Say "No migration required" if true.>

## Contributors

<Contributors derived from commits in the release range.>
