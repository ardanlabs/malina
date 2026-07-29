## Summary

Describe the problem and the smallest change that solves it.

## Validation

- [ ] Focused tests pass
- [ ] `go test ./...` passes (native-dependent skips noted below)
- [ ] `go vet ./...`, `go fix -diff ./...`, and `staticcheck -checks=all ./...` pass
- [ ] BUI changes pass `npm run build`

## Lifecycle and compatibility

- [ ] The one-resident-model, one-generation, bounded-queue invariants remain intact
- [ ] Queue cancellation, unload waiting, and poisoning behavior are tested if affected
- [ ] Native ABI/platform/backend and `stable-diffusion.cpp` compatibility effects are described
- [ ] Not applicable items are explained below

Native assets used, skipped checks, and compatibility notes:
