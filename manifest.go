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
	// Effect is the action's declared effect level: "read", "local",
	// "history" or "network". An empty value keeps the conservative rule,
	// under which every process, write and terminal action asks.
	Effect string `json:"effect,omitempty"`
}
type View struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Glyph string `json:"glyph"`
	Page  string `json:"page"`
	Pane  bool   `json:"pane,omitempty"`
	// Follows makes a window take the selected workspace as a pane does,
	// with a pin the person may set. A window without it pins at opening.
	Follows bool `json:"follows,omitempty"`
	// Opens lists the kinds of file this view opens when the person follows
	// a link to one in Dinah. Only a window that is neither a pane nor
	// following may declare it, since a file belongs to one workspace and
	// such a window keeps the workspace it opened with.
	Opens []FileType `json:"opens,omitempty"`
}

// FileType is a kind of file a view opens: an IANA media type, written in
// lower case with no parameters, and the file extensions that have it.
type FileType struct {
	MediaType  string   `json:"mediaType"`
	Extensions []string `json:"extensions"`
}

// Limits of the file types a manifest may declare.
const (
	FileTypesPerView      = 8
	FileTypesPerManifest  = 32
	ExtensionsPerFileType = 16
	MediaTypeLimit        = 127
)

// A media type a view may open is a registered top-level type and a subtype
// made of the characters RFC 6838 allows, in lower case. A wildcard such as
// image/* is not a media type and does not match.
var mediaType = regexp.MustCompile(`^(application|audio|font|image|model|text|video)/[a-z0-9][a-z0-9!#$&^_.+-]*$`)

// A file extension is a dot and one segment of at most 31 lower-case letters,
// digits, underscores, plus signs and hyphens.
var fileExtension = regexp.MustCompile(`^\.[a-z0-9][a-z0-9_+-]{0,30}$`)

// genericMediaTypes describe any file rather than a kind of file, so no view
// may claim them.
var genericMediaTypes = map[string]bool{"text/plain": true, "application/octet-stream": true}

// ValidFileType says whether a file type may be declared by a view.
func ValidFileType(t FileType) bool {
	if len(t.MediaType) > MediaTypeLimit || !mediaType.MatchString(t.MediaType) || genericMediaTypes[t.MediaType] {
		return false
	}
	if len(t.Extensions) == 0 || len(t.Extensions) > ExtensionsPerFileType {
		return false
	}
	for _, e := range t.Extensions {
		if !fileExtension.MatchString(e) {
			return false
		}
	}
	return true
}

// MinimumSDK is the earliest release of this SDK whose hosts accept the
// manifest. A host rejects a manifest member it does not know, so a package
// that uses a member added within contract 1 installs only in a host built
// with the release that added it or a later one.
func (m Manifest) MinimumSDK() string {
	for _, a := range m.Actions {
		if a.Effect != "" {
			return "0.2.0"
		}
	}
	for _, v := range m.Views {
		if v.Follows || len(v.Opens) > 0 {
			return "0.2.0"
		}
	}
	return "0.1.0"
}

// Effect levels an action may declare.
const (
	EffectRead    = "read"
	EffectLocal   = "local"
	EffectHistory = "history"
	EffectNetwork = "network"
)

func ValidEffect(effect string) bool {
	switch effect {
	case "", EffectRead, EffectLocal, EffectHistory, EffectNetwork:
		return true
	}
	return false
}

// EffectAsks says whether an action at this effect level is one Dinah asks
// about before an agent runs it, and one Show Me First reviews. Reads and
// reversible local changes run at once; rewriting history, reaching a
// network and an undeclared level ask.
func EffectAsks(effect string) bool {
	return effect == "" || effect == EffectHistory || effect == EffectNetwork
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
		if !localID.MatchString(a.Name) || actions[a.Name] || a.Title == "" || !json.Valid(a.InputSchema) || !json.Valid(a.OutputSchema) || (a.Replay != "pure" && a.Replay != "once") || !ValidEffect(a.Effect) {
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
	types, extensions, declared := map[string]bool{}, map[string]bool{}, 0
	for _, v := range m.Views {
		if !localID.MatchString(v.ID) || views[v.ID] || v.Title == "" || !SafePath(v.Page) || !strings.HasSuffix(v.Page, ".html") {
			return errors.New("invalid view")
		}
		views[v.ID] = true
		if len(v.Opens) == 0 {
			continue
		}
		if v.Pane || v.Follows {
			return fmt.Errorf("view %s opens files, so it cannot be a pane or follow the selected workspace", v.ID)
		}
		if !Has(m.Permissions, "workspace.read") {
			return fmt.Errorf("view %s opens files, which needs the workspace.read permission", v.ID)
		}
		if len(v.Opens) > FileTypesPerView {
			return fmt.Errorf("view %s opens more than %d file types", v.ID, FileTypesPerView)
		}
		for _, t := range v.Opens {
			if !ValidFileType(t) {
				return fmt.Errorf("view %s declares an invalid file type %q", v.ID, t.MediaType)
			}
			if types[t.MediaType] {
				return fmt.Errorf("media type %s is declared more than once", t.MediaType)
			}
			types[t.MediaType] = true
			for _, e := range t.Extensions {
				if extensions[e] {
					return fmt.Errorf("file extension %s is declared more than once", e)
				}
				extensions[e] = true
			}
			declared++
		}
	}
	if declared > FileTypesPerManifest {
		return fmt.Errorf("an extension may declare at most %d file types", FileTypesPerManifest)
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
