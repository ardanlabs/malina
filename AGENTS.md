# Malina contributor notes

Malina is a Go SDK, CLI, and local HTTP service for image generation through
`stable-diffusion.cpp`.

## Layout

- `sdk/malina`: public lifecycle, admission, and generation API.
- `sdk/malina/model`: reusable model ownership and PNG encoding.
- `sdk/malina/sd`: thin FFI mirror of the native C API; keep it close to
  upstream rather than adding policy here.
- `sdk/tools`: native-library and model download/catalog tooling.
- `cmd/malina`: CLI and subcommands.
- `cmd/server`: HTTP API, model service, and embedded browser UI (BUI).
- `examples`: focused SDK examples.

## Lifecycle invariants

- A loaded model owns one reusable, exclusive `sd.Context`; keep
  `FreeParamsImmediately=false` so repeated generation remains valid.
- Only one model is resident and one native generation runs at a time. Request
  admission uses a bounded queue.
- Queue waits are cancellable. Once native generation starts it is synchronous
  and non-cancellable.
- Unload waits for in-flight generation and prevents queued work from entering
  the backend.
- A native generation error poisons the high-level lifecycle; validation and
  PNG-encoding errors do not.

## Validation

Use the Go version in `.go-version` (the authoritative toolchain pin).

```sh
go test ./...
go test -race ./...
gofmt -w <changed-go-files>
go fix -diff ./...
go vet ./...
staticcheck -checks=all ./...
go build ./...
```

Native tests skip when their assets are unavailable. To run them, set
`MALINA_LIB` and the relevant model variables, including `MALINA_TEST_MODEL`.
The makefile also provides `make test`, `make lint`, and
`make pull-test-assets`.

For the BUI:

```sh
cd cmd/server/api/frontends/bui
npm ci
npm run build
npm run dev
```

Keep changes small, preserve the lifecycle guarantees, and add focused tests
for behavior changes.
