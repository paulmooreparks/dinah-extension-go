//go:build windows

package extension

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestFailedRemovalRemainsRetryable(t *testing.T) {
	folder, key := packageFixture(t)
	manifest, err := Sign(folder, key)
	if err != nil {
		t.Fatal(err)
	}
	store := Store{Root: filepath.Join(t.TempDir(), "extensions")}
	if _, err = store.Install(folder, "fixture", Approval{Fingerprint(manifest.Publisher.Key), manifest.Signature}); err != nil {
		t.Fatal(err)
	}
	name, _ := windows.UTF16PtrFromString(filepath.Join(store.Path(manifest), "backend.exe"))
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Remove(manifest.ID); err == nil {
		windows.CloseHandle(handle)
		t.Fatal("removed a file without delete sharing")
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		windows.CloseHandle(handle)
		t.Fatal("failed removal disappeared from the registry", err)
	}
	windows.CloseHandle(handle)
	if err = store.Remove(manifest.ID); err != nil {
		t.Fatal("removal could not be retried", err)
	}
}
