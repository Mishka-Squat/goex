package fsex

import (
	"fmt"
	"io/fs"
)

// Mount overlays several filesystems: a name is looked up in each mounted
// FS in mount order and the first match wins.
type Mount struct {
	fs []fs.FS
}

var _ fs.ReadFileFS = (*Mount)(nil)

func (m *Mount) Mount(fsys fs.FS) {
	m.fs = append(m.fs, fsys)
}

func (m *Mount) Open(name string) (fs.File, error) {
	for _, lib := range m.fs {
		if f, err := lib.Open(name); err == nil {
			return f, nil
		}
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *Mount) ReadFile(name string) ([]byte, error) {
	for _, lib := range m.fs {
		if src, err := fs.ReadFile(lib, name); err == nil {
			return src, nil
		}
	}

	return nil, fmt.Errorf("file %s: not found in the mount: %w", name, fs.ErrNotExist)
}
