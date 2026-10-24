//go:build windows

package extension

import (
	"golang.org/x/sys/windows"
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
	over := &windows.Overlapped{}
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, over)
	if err != nil {
		f.Close()
		return nil, err
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, over); f.Close() }, nil
}
func replaceFile(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
