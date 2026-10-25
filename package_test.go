package extension

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func packageFixture(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	root := t.TempDir()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	m := Manifest{ID: "test.extension", Version: "0.1.0", Contract: 1, Title: "Fixture", Publisher: Publisher{Name: "Fixture"}, Backends: map[string]string{"windows": "backend.exe", "linux": "backend.exe"}, Permissions: []string{"workspace.read"}, Actions: []Action{{Name: "status", Title: "Status", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Replay: "pure"}}, Views: []View{{ID: "main", Title: "Main", Page: "index.html"}}}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(root, ManifestFile), data, 0600)
	os.WriteFile(filepath.Join(root, "backend.exe"), []byte("not executable"), 0600)
	os.WriteFile(filepath.Join(root, "index.html"), []byte("<p>Fixture</p>"), 0600)
	return root, key
}
func TestSignedInventoryAndPinnedPublisher(t *testing.T) {
	root, key := packageFixture(t)
	m, err := Sign(root, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(root); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: filepath.Join(t.TempDir(), "extensions")}
	if _, err = s.Install(root, "fixture", Approval{}); err == nil {
		t.Fatal("installed without consent fingerprint")
	}
	if _, err = s.Install(root, "fixture", Approval{Fingerprint(m.Publisher.Key), m.Signature}); err != nil {
		t.Fatal(err)
	}
	if err = s.Review(m.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Install(root, "fixture", Approval{Fingerprint(m.Publisher.Key), m.Signature}); err != nil {
		t.Fatal(err)
	}
	installed, _ := s.List()
	if len(installed) != 1 || !installed[0].ReviewCommands {
		t.Fatal("update lost command review preference")
	}
	os.WriteFile(filepath.Join(root, "index.html"), []byte("changed"), 0600)
	if _, err = Verify(root); err == nil {
		t.Fatal("accepted modified payload")
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	m, err = Sign(root, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Install(root, "other", Approval{Fingerprint(m.Publisher.Key), m.Signature}); err == nil {
		t.Fatal("changed publisher key silently")
	}
	if err = s.Remove("../escape"); err == nil {
		t.Fatal("unsafe removal")
	}
	if err = s.Remove("test.extension"); err != nil {
		t.Fatal(err)
	}
}
func TestPortablePackageNames(t *testing.T) {
	for _, p := range []string{"NUL", "sub/CON.txt", "COM1.js", "LPT²", "trailing. ", "part/..", "bad:name", "bad?name", "line\nname", "a\\b", "../outside", "/root"} {
		if SafePath(p) {
			t.Errorf("accepted unsafe name %q", p)
		}
	}
	for _, p := range []string{"web/index.html", "bin/extension.exe", "normal name.txt", "icon-dark.svg"} {
		if !SafePath(p) {
			t.Errorf("refused safe name %q", p)
		}
	}
}
func TestZipRejectsTraversalAndCollisions(t *testing.T) {
	for _, names := range [][]string{{"../escape"}, {"A.js", "a.js"}, {"/absolute"}, {"a\\b"}} {
		archive := filepath.Join(t.TempDir(), "extension.zip")
		f, _ := os.Create(archive)
		w := zip.NewWriter(f)
		for _, p := range names {
			entry, _ := w.Create(p)
			entry.Write([]byte("x"))
		}
		w.Close()
		f.Close()
		if err := Unzip(archive, t.TempDir()); err == nil {
			t.Fatalf("accepted %v", names)
		}
	}
}
func TestEffectAndFollowsAreSignedAndUnknownFieldsRejected(t *testing.T) {
	root, key := packageFixture(t)
	m, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Permissions = append(m.Permissions, "process.run:git")
	m.Actions[0].Permissions = []string{"process.run:git"}
	m.Actions[0].Replay = "once"
	m.Actions[0].Effect = EffectLocal
	m.Actions = append(m.Actions, Action{Name: "push", Title: "Push", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Permissions: []string{"process.run:git"}, Replay: "once", Effect: EffectNetwork})
	m.Views = append(m.Views, View{ID: "window", Title: "Window", Page: "index.html", Follows: true})
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(root, ManifestFile), data, 0600)
	signed, err := Sign(root, key)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(root)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Signature != signed.Signature || verified.Actions[0].Effect != EffectLocal || verified.Actions[1].Effect != EffectNetwork || !verified.Views[1].Follows {
		t.Fatal("effect levels or follows did not survive signing")
	}
	if !EffectAsks("") || !EffectAsks(EffectHistory) || !EffectAsks(EffectNetwork) || EffectAsks(EffectRead) || EffectAsks(EffectLocal) {
		t.Fatal("effect consent rule is wrong")
	}
	m.Actions[0].Effect = "harmless"
	if m.Validate() == nil {
		t.Fatal("accepted an unknown effect level")
	}
	// The signed JSON with a field this contract does not define is still refused.
	var loose map[string]any
	json.Unmarshal(data, &loose)
	loose["shared"] = true
	data, _ = json.Marshal(loose)
	os.WriteFile(filepath.Join(root, ManifestFile), data, 0600)
	if _, err = ReadManifest(root); err == nil {
		t.Fatal("accepted an unknown manifest field")
	}
}
func TestDiffSpecValidation(t *testing.T) {
	good := DiffSpec{Key: "README.md@index", Title: "README.md", Detail: "Staged", Path: "README.md", Language: "markdown", Before: "a\n", After: "b\n", Mode: "split", Actions: []DiffAction{{ID: "stage-hunk", Label: "Stage Hunk", Glyph: "plus", Scope: "hunk", Primary: true}, {ID: "discard", Label: "Discard", Scope: "line", Danger: true}}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*DiffSpec){
		"missing key":        func(d *DiffSpec) { d.Key = " " },
		"long key":           func(d *DiffSpec) { d.Key = string(make([]byte, DiffKeyLimit+1)) },
		"control in title":   func(d *DiffSpec) { d.Title = "a\x00b" },
		"oversized side":     func(d *DiffSpec) { d.After = string(make([]byte, DiffSideLimit+1)) },
		"unknown mode":       func(d *DiffSpec) { d.Mode = "three-way" },
		"bad action id":      func(d *DiffSpec) { d.Actions[0].ID = "Stage Hunk" },
		"repeated action id": func(d *DiffSpec) { d.Actions[1].ID = d.Actions[0].ID },
		"bad scope":          func(d *DiffSpec) { d.Actions[0].Scope = "repository" },
		"empty label":        func(d *DiffSpec) { d.Actions[0].Label = "" },
		"bad language":       func(d *DiffSpec) { d.Language = "C Sharp" },
		"too many actions": func(d *DiffSpec) {
			for i := 0; i <= DiffActionsLimit; i++ {
				d.Actions = append(d.Actions, DiffAction{ID: fmt.Sprintf("a%d", i), Label: "A", Scope: "file"})
			}
		},
	} {
		spec := good
		spec.Actions = append([]DiffAction{}, good.Actions...)
		change(&spec)
		if spec.Validate() == nil {
			t.Errorf("accepted a diff with %s", name)
		}
	}
	sides := DiffSpec{Key: "k", Title: "t", Before: string(make([]byte, DiffSideLimit)), After: ""}
	if err := sides.Validate(); err != nil {
		t.Fatal("refused a side at the limit", err)
	}
}
func TestProcessActionsCannotClaimPureReplay(t *testing.T) {
	root, _ := packageFixture(t)
	m, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Permissions = append(m.Permissions, "process.run:git")
	m.Actions[0].Permissions = []string{"process.run:git"}
	if m.Validate() == nil {
		t.Fatal("process action bypassed once policy")
	}
}
