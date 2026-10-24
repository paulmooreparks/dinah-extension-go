package extension

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Installation struct {
	Manifest       Manifest  `json:"manifest"`
	Origin         string    `json:"origin"`
	Installed      time.Time `json:"installed"`
	Enabled        bool      `json:"enabled"`
	ReviewCommands bool      `json:"reviewCommands,omitempty"`
}
type Store struct{ Root string }
type Approval struct {
	Fingerprint string `json:"fingerprint"`
	Signature   string `json:"signature"`
}

type Preview struct {
	Manifest    Manifest      `json:"manifest"`
	Fingerprint string        `json:"fingerprint"`
	Existing    *Installation `json:"existing,omitempty"`
}

func (s Store) Path(m Manifest) string { return filepath.Join(s.Root, m.ID, m.Version) }
func (s Store) read() (map[string]Installation, error) {
	all := map[string]Installation{}
	data, err := os.ReadFile(filepath.Join(s.Root, "registry.json"))
	if os.IsNotExist(err) {
		return all, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 4<<20 {
		return nil, errors.New("extension registry exceeds limit")
	}
	if err = json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for id, installed := range all {
		if installed.Manifest.ID != id || installed.Manifest.Validate() != nil {
			return nil, errors.New("extension registry contains an invalid manifest")
		}
	}
	return all, nil
}
func (s Store) write(all map[string]Installation) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(s.Root, ".registry-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(0600); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(name, filepath.Join(s.Root, "registry.json"))
}
func (s Store) List() ([]Installation, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return nil, err
	}
	list := []Installation{}
	for _, v := range all {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Manifest.ID < list[j].Manifest.ID })
	return list, nil
}
func (s Store) Inspect(folder string) (Preview, error) {
	m, err := Verify(folder)
	if err != nil {
		return Preview{}, err
	}
	out := Preview{Manifest: m, Fingerprint: Fingerprint(m.Publisher.Key)}
	list, err := s.List()
	if err != nil {
		return out, err
	}
	for _, v := range list {
		if v.Manifest.ID == m.ID {
			out.Existing = &v
		}
	}
	return out, nil
}
func (s Store) Install(folder, origin string, approval Approval) (Installation, error) {
	m, err := Verify(folder)
	if err != nil {
		return Installation{}, err
	}
	if approval.Fingerprint == "" || approval.Fingerprint != Fingerprint(m.Publisher.Key) || approval.Signature != m.Signature {
		return Installation{}, errors.New("approve the displayed publisher fingerprint before installation")
	}
	unlock, err := s.lock()
	if err != nil {
		return Installation{}, err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return Installation{}, err
	}
	if old, ok := all[m.ID]; ok && old.Manifest.Publisher.Key != m.Publisher.Key {
		return Installation{}, errors.New("publisher key changed; explicit removal and a new trust decision are required")
	}
	target := s.Path(m)
	if _, err = os.Stat(target); err == nil {
		installed, err := Verify(target)
		if err != nil {
			return Installation{}, err
		}
		if installed.Signature != m.Signature {
			return Installation{}, errors.New("an installed version cannot be replaced with different contents")
		}
	} else if !os.IsNotExist(err) {
		return Installation{}, err
	} else {
		stage, err := os.MkdirTemp(s.Root, ".stage-*")
		if err != nil {
			return Installation{}, err
		}
		defer os.RemoveAll(stage)
		if err = CopyPackage(folder, stage); err != nil {
			return Installation{}, err
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return Installation{}, err
		}
		if err = os.Rename(stage, target); err != nil {
			return Installation{}, err
		}
	}
	v := Installation{Manifest: m, Origin: origin, Installed: time.Now().UTC(), Enabled: true}
	if old, ok := all[m.ID]; ok {
		v.ReviewCommands = old.ReviewCommands
	}
	all[m.ID] = v
	return v, s.write(all)
}
func (s Store) Review(id string, enabled bool) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	v, ok := all[id]
	if !ok {
		return errors.New("extension is not installed")
	}
	v.ReviewCommands = enabled
	all[id] = v
	return s.write(all)
}
func (s Store) Enable(id string, enabled bool) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	v, ok := all[id]
	if !ok {
		return errors.New("extension is not installed")
	}
	v.Enabled = enabled
	all[id] = v
	return s.write(all)
}
func (s Store) Remove(id string) error {
	if !identifier.MatchString(id) {
		return errors.New("invalid extension ID")
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := all[id]; !ok {
		return errors.New("extension is not installed")
	}
	target := filepath.Join(s.Root, id)
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	base, err := filepath.Abs(s.Root)
	if err != nil || filepath.Dir(absolute) != base {
		return fmt.Errorf("removal is outside extension store")
	}
	// Keep a failed removal visible and retryable. In particular, Windows
	// may refuse deletion while a backend still holds its executable open.
	if err = os.RemoveAll(absolute); err != nil {
		return err
	}
	delete(all, id)
	return s.write(all)
}
