//go:build !windows

package extension

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func (s Store) lock() (func(), error) {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.Root, "registry.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}
func replaceFile(source, target string) error { return os.Rename(source, target) }
