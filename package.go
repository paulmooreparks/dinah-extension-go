package extension

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxPackageBytes = 256 << 20
const MaxFiles = 4096

func ReadManifest(root string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(filepath.Join(root, ManifestFile))
	if err != nil {
		return m, err
	}
	if len(data) > 1<<20 {
		return m, errors.New("manifest exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil {
		return m, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return m, errors.New("trailing manifest data")
	}
	return m, m.Validate()
}
func signedBytes(m Manifest) ([]byte, error) { m.Signature = ""; return json.Marshal(m) }
func Fingerprint(key string) string {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func Inventory(root string) (map[string]string, error) {
	files := map[string]string{}
	folded := map[string]bool{}
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("extension packages cannot contain symlinks")
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !SafePath(rel) {
			return fmt.Errorf("unsafe package path %q", rel)
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return errors.New("package a release folder, not a Git checkout")
			}
			return nil
		}
		if rel == ManifestFile {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("package contains a nonregular file")
		}
		if info.Size() > 128<<20 {
			return errors.New("package file exceeds 128 MiB")
		}
		total += info.Size()
		if total > MaxPackageBytes || len(files) >= MaxFiles {
			return errors.New("package exceeds limits")
		}
		key := strings.ToLower(rel)
		if folded[key] {
			return errors.New("package has case-colliding paths")
		}
		folded[key] = true
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		files[rel] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return files, err
}
func Sign(root string, key ed25519.PrivateKey) (Manifest, error) {
	m, err := ReadManifest(root)
	if err != nil {
		return m, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return m, errors.New("invalid signing key")
	}
	m.Publisher.Key = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	m.Files, err = Inventory(root)
	if err != nil {
		return m, err
	}
	data, err := signedBytes(m)
	if err != nil {
		return m, err
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, data))
	data, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	return m, os.WriteFile(filepath.Join(root, ManifestFile), append(data, '\n'), 0644)
}
func Verify(root string) (Manifest, error) {
	m, err := ReadManifest(root)
	if err != nil {
		return m, err
	}
	key, err := base64.StdEncoding.DecodeString(m.Publisher.Key)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return m, errors.New("invalid publisher public key")
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return m, errors.New("invalid signature encoding")
	}
	data, err := signedBytes(m)
	if err != nil {
		return m, err
	}
	if !ed25519.Verify(key, data, signature) {
		return m, errors.New("extension signature does not match its publisher key")
	}
	actual, err := Inventory(root)
	if err != nil {
		return m, err
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(m.Files)
	if !bytes.Equal(a, b) {
		return m, errors.New("extension payload differs from signed inventory")
	}
	for _, p := range m.Backends {
		if _, ok := actual[p]; !ok {
			return m, fmt.Errorf("missing backend %s", p)
		}
	}
	for _, v := range m.Views {
		if _, ok := actual[v.Page]; !ok {
			return m, fmt.Errorf("missing view %s", v.Page)
		}
	}
	return m, nil
}
func CopyPackage(source, target string) error {
	m, err := Verify(source)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(target, 0700); err != nil {
		return err
	}
	names := []string{ManifestFile}
	for p := range m.Files {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		data, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		dest := filepath.Join(target, filepath.FromSlash(p))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		mode := fs.FileMode(0644)
		for _, exe := range m.Backends {
			if exe == p {
				mode = 0755
			}
		}
		if err = os.WriteFile(dest, data, mode); err != nil {
			return err
		}
	}
	_, err = Verify(target)
	return err
}
func Unzip(source, target string) error {
	z, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer z.Close()
	var total uint64
	seen := map[string]bool{}
	if len(z.File) > MaxFiles+64 {
		return errors.New("archive has too many entries")
	}
	for _, f := range z.File {
		p := strings.TrimSuffix(f.Name, "/")
		if !SafePath(p) || f.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe archive path")
		}
		key := strings.ToLower(p)
		if seen[key] {
			return errors.New("duplicate archive path")
		}
		seen[key] = true
		total += f.UncompressedSize64
		if total > MaxPackageBytes {
			return errors.New("archive exceeds size limit")
		}
		dest := filepath.Join(target, filepath.FromSlash(p))
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(in, 128<<20+1))
		in.Close()
		if err != nil {
			return err
		}
		if len(data) > 128<<20 {
			return errors.New("archive file exceeds limit")
		}
		if err = os.WriteFile(dest, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
