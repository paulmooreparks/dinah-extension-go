// Package extension defines Dinah's independently versioned extension contract.
package extension

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const ContractVersion = 1
const ManifestFile = "dinah-extension.json"
const ScopeMeta = "com.parkscomputing.dinah/host"

type Publisher struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}
type Action struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Permissions  []string        `json:"permissions"`
	Replay       string          `json:"replay"`
}
type View struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Glyph string `json:"glyph"`
	Page  string `json:"page"`
	Pane  bool   `json:"pane,omitempty"`
}
type Command struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Glyph  string `json:"glyph,omitempty"`
	Action string `json:"action,omitempty"`
	View   string `json:"view,omitempty"`
}
type Manifest struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Contract    int               `json:"contract"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Publisher   Publisher         `json:"publisher"`
	Backends    map[string]string `json:"backends"`
	Permissions []string          `json:"permissions"`
	Actions     []Action          `json:"actions"`
	Views       []View            `json:"views"`
	Commands    []Command         `json:"commands,omitempty"`
	Files       map[string]string `json:"files"`
	Signature   string            `json:"signature"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`)
var localID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var version = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[a-zA-Z0-9.-]+)?$`)

func SafePath(p string) bool {
	if p == "" || len(p) > 512 || strings.ContainsAny(p, "\\:<>\"|?*\x00") || strings.HasPrefix(p, "/") || path.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if strings.TrimRight(part, " .") != part || strings.IndexFunc(part, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
			return false
		}
		stem, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || regexp.MustCompile(`^(COM|LPT)[1-9¹²³]$`).MatchString(stem) {
			return false
		}
	}
	return true
}
func Has(list []string, value string) bool {
	for _, x := range list {
		if x == value {
			return true
		}
	}
	return false
}
func ValidPermission(p string) bool {
	switch p {
	case "workspace.read", "workspace.write", "workspace.decorate", "terminal.open", "notification.post", "network":
		return true
	}
	return strings.HasPrefix(p, "process.run:") && localID.MatchString(strings.TrimPrefix(p, "process.run:"))
}
func Effectful(permissions []string) bool {
	for _, p := range permissions {
		if p == "workspace.write" || p == "terminal.open" || strings.HasPrefix(p, "process.run:") {
			return true
		}
	}
	return false
}
func (m Manifest) Validate() error {
	if !identifier.MatchString(m.ID) || !version.MatchString(m.Version) || m.Contract != ContractVersion || strings.TrimSpace(m.Title) == "" || len(m.Title) > 100 || strings.TrimSpace(m.Publisher.Name) == "" {
		return errors.New("invalid extension identity, publisher or contract")
	}
	if len(m.Actions) == 0 || len(m.Actions) > 100 || len(m.Views) > 20 || len(m.Permissions) > 32 {
		return errors.New("invalid extension contribution count")
	}
	seen := map[string]bool{}
	for _, p := range m.Permissions {
		if !ValidPermission(p) || seen[p] {
			return fmt.Errorf("invalid or repeated permission %q", p)
		}
		seen[p] = true
	}
	for os, p := range m.Backends {
		if (os != "windows" && os != "linux" && os != "darwin") || !SafePath(p) {
			return errors.New("invalid backend path")
		}
	}
	if len(m.Backends) == 0 {
		return errors.New("a backend is required")
	}
	actions := map[string]bool{}
	for _, a := range m.Actions {
		if !localID.MatchString(a.Name) || actions[a.Name] || a.Title == "" || !json.Valid(a.InputSchema) || !json.Valid(a.OutputSchema) || (a.Replay != "pure" && a.Replay != "once") {
			return fmt.Errorf("invalid action %q", a.Name)
		}
		actions[a.Name] = true
		for _, p := range a.Permissions {
			if !Has(m.Permissions, p) {
				return fmt.Errorf("action %s requests undeclared permission %s", a.Name, p)
			}
		}
		if Effectful(a.Permissions) && a.Replay != "once" {
			return fmt.Errorf("process and write actions must use once replay: %s", a.Name)
		}
	}
	views := map[string]bool{}
	for _, v := range m.Views {
		if !localID.MatchString(v.ID) || views[v.ID] || v.Title == "" || !SafePath(v.Page) || !strings.HasSuffix(v.Page, ".html") {
			return errors.New("invalid view")
		}
		views[v.ID] = true
	}
	commands := map[string]bool{}
	for _, c := range m.Commands {
		if !localID.MatchString(c.ID) || commands[c.ID] || c.Title == "" || ((c.Action == "") == (c.View == "")) || c.Action != "" && !actions[c.Action] || c.View != "" && !views[c.View] {
			return errors.New("invalid command")
		}
		commands[c.ID] = true
	}
	return nil
}
