//go:build windows

package loader

import (
	"fmt"
	"path/filepath"

	"github.com/jupiterrider/ffi"
	"golang.org/x/sys/windows"
)

func load(filename string) (ffi.Lib, error) {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return ffi.Lib{}, fmt.Errorf("%s: resolve absolute library path: %w", filename, err)
	}

	handle, err := windows.LoadLibraryEx(absolute, 0, windows.LOAD_WITH_ALTERED_SEARCH_PATH)
	if err != nil {
		return ffi.Lib{}, fmt.Errorf("%s: error loading library: %w", absolute, err)
	}

	return ffi.Lib{Addr: uintptr(handle)}, nil
}
