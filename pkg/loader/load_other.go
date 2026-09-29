//go:build !windows

package loader

import "github.com/jupiterrider/ffi"

func load(filename string) (ffi.Lib, error) {
	return ffi.Load(filename)
}
