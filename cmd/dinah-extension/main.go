package main

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	extension "github.com/paulmooreparks/dinah-extension-go"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: dinah-extension keygen KEY | sign FOLDER KEY | verify FOLDER | zip FOLDER OUTPUT")
	}
	switch os.Args[1] {
	case "keygen":
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(os.Args[2], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(base64.StdEncoding.EncodeToString(key))
		return err
	case "sign":
		if len(os.Args) != 4 {
			return fmt.Errorf("sign requires folder and private key")
		}
		data, err := os.ReadFile(os.Args[3])
		if err != nil {
			return err
		}
		key, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return err
		}
		m, err := extension.Sign(os.Args[2], key)
		if err == nil {
			fmt.Println(m.ID, m.Version, extension.Fingerprint(m.Publisher.Key))
		}
		return err
	case "verify":
		m, err := extension.Verify(os.Args[2])
		if err == nil {
			fmt.Println(m.ID, m.Version, extension.Fingerprint(m.Publisher.Key))
		}
		return err
	case "zip":
		if len(os.Args) != 4 {
			return fmt.Errorf("zip requires folder and output")
		}
		m, err := extension.Verify(os.Args[2])
		if err != nil {
			return err
		}
		names := []string{extension.ManifestFile}
		for p := range m.Files {
			names = append(names, p)
		}
		sort.Strings(names)
		f, err := os.Create(os.Args[3])
		if err != nil {
			return err
		}
		defer f.Close()
		z := zip.NewWriter(f)
		for _, p := range names {
			w, err := z.Create(p)
			if err != nil {
				return err
			}
			in, err := os.Open(filepath.Join(os.Args[2], filepath.FromSlash(p)))
			if err != nil {
				return err
			}
			_, err = io.Copy(w, in)
			in.Close()
			if err != nil {
				return err
			}
		}
		return z.Close()
	}
	return fmt.Errorf("unknown command")
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
