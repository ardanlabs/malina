# Contributing to Malina

Thank you for helping improve Malina. Keep pull requests focused, explain the
behavioral reason for a change, and include tests where behavior changes.

## Setup

1. Fork and clone the repository.
2. Install the exact Go version recorded in `.go-version`; that file is the
   authoritative toolchain source.
3. Download modules with `go mod download`.
4. Run `go test ./...` to establish a baseline.

Nix may be used if it is convenient, but it is not required.

Most unit tests do not need a native installation. Tests that require
`stable-diffusion.cpp` skip when the native library or model is absent. For
native integration work, install a compatible bundle (for example,
`go run ./cmd/malina libs pull --lib ./lib`), set `MALINA_LIB` to its directory,
and set `MALINA_TEST_MODEL` or the model-specific environment variable used by
the test. Models and native bundles are large and are not committed.

## Validate changes

Run the focused package tests while developing, then the repository checks:

```sh
go test ./...
go test -race ./...
gofmt -w <changed-go-files>
go fix -diff ./...
go vet ./...
staticcheck -checks=all ./...
go build ./...
```

`make test` combines the normal project checks. `make pull-test-assets` is
available when the full native test assets are needed. Do not hide diagnostics
or commit generated test images, downloaded models, native bundles, or build
outputs.

For BUI changes, use the committed npm lockfile:

```sh
cd cmd/server/api/frontends/bui
npm ci
npm run build
npm run dev  # local development server
```

## Pull requests and releases

- Prefer one coherent change over broad refactoring.
- Preserve the one-model, one-generation, bounded-queue lifecycle guarantees.
- Call out platform/backend effects and native ABI changes explicitly.
- Update tests and user documentation together with changed behavior.
- Avoid unrelated dependency or formatting churn.

Maintainers assign versions, create tags, and publish releases. Contributors
should describe compatibility or breaking-change implications, but should not
bump release versions or regenerate release artifacts unless a maintainer asks
for it. Native ABI updates should identify the compatible
`stable-diffusion.cpp` revision and any provenance/checksum implications.
