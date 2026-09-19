package fsex

import (
	"fmt"
	"io/fs"
)

type Mount struct {
	fs []fs.FS
}

func (m *Mount) Mount(fsys fs.FS) {
	m.fs = append(m.fs, fsys)
}

func (m *Mount) ReadFile(name string) ([]byte, error) {
	for _, lib := range m.fs {
		if src, err := fs.ReadFile(lib, name); err == nil {
			return src, nil
		}
	}

	return nil, fmt.Errorf("file %s: not found in the mount", name)
}
