// Package models provides utilities for downloading curated stable-diffusion
// model bundles. A bundle is the set of files (diffusion model, VAE, text
// encoders) that the C API requires to construct a single Context.
//
// Bundle curation lives here so first-time users can run a working
// text-to-image pipeline with one command. Downstream consumers (e.g. kronk's
// model server) are expected to layer their own catalog / lifecycle on top
// of, or alongside, this package.
package models
