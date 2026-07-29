# Malina Nix phase

The flake packages the Go `cmd/malina` CLI and its committed, embedded BUI
static output. It intentionally does **not** package stable-diffusion.cpp,
native inference backends, or models. The wrapper's `libffi` and C++ runtime
libraries are generic runtime support; they are not the native inference
bundle. Set `MALINA_LIB` explicitly to a host-provided native library path when
native inference is needed.

On Linux, a `MALINA_LIB` value is added to `LD_LIBRARY_PATH` only when it names
an existing absolute directory. A library-file path remains available to
Malina itself but is not added to the loader search path. No host path is
captured in the Nix store.

The default and `cpu` development shells provide Go 1.26, Go tooling,
gomod2nix, pkg-config, libffi/C++ runtime support, and Node 22. The optional
`vulkan` shell adds Vulkan headers and the loader. These shells do not imply
that a native backend is bundled.

No Node build runs in the Go derivation because the committed BUI output is
embedded by the Go source. Regenerate that output outside Nix. Moving BUI
regeneration into the derivation would require explicitly and reproducibly
modeling the npm dependency graph.

From `zarf/nix`, regenerate the module manifest with the live gomod2nix CLI:

```sh
(cd ../.. && gomod2nix generate --dir . --outdir zarf/nix)
```

Shell entry never changes `gomod2nix.toml`; regeneration is always explicit.
