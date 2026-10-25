package extension

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// DiffSpec is the payload of the open-diff host command, by which a frame
// opens a diff in Dinah's Editor. The host validates it with Validate before
// opening the view, and routes each action the person takes back to the frame
// as a command event {id: "diff/<action id>", diff: key, where}.
type DiffSpec struct {
	Key      string       `json:"key"`
	Title    string       `json:"title"`
	Detail   string       `json:"detail,omitempty"`
	Path     string       `json:"path,omitempty"`
	Language string       `json:"language,omitempty"`
	Before   string       `json:"before"`
	After    string       `json:"after"`
	Mode     string       `json:"mode,omitempty"`
	ReadOnly bool         `json:"readOnly,omitempty"`
	Actions  []DiffAction `json:"actions,omitempty"`
}

// DiffAction is an action the Editor shows on a diff's file, hunks or lines.
type DiffAction struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Glyph   string `json:"glyph,omitempty"`
	Scope   string `json:"scope"`
	Primary bool   `json:"primary,omitempty"`
	Danger  bool   `json:"danger,omitempty"`
}

// Limits of a diff specification. A side is limited to 2 MiB, which is the
// same bound host.read and host.write apply to a file.
const (
	DiffSideLimit    = 2 << 20
	DiffActionsLimit = 20
	DiffKeyLimit     = 200
	DiffTitleLimit   = 200
	DiffDetailLimit  = 500
	DiffPathLimit    = 1024
)

var diffActionID = regexp.MustCompile(`^[-a-z0-9]{1,64}$`)
var diffGlyph = regexp.MustCompile(`^[a-z0-9-]{0,40}$`)
var diffLanguage = regexp.MustCompile(`^[a-z0-9+#.-]{0,40}$`)

func plainText(s string, limit int) bool {
	return len(s) <= limit && strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) }) < 0
}

func (d DiffSpec) Validate() error {
	if strings.TrimSpace(d.Key) == "" || !plainText(d.Key, DiffKeyLimit) {
		return errors.New("a diff needs a key of at most 200 characters")
	}
	if strings.TrimSpace(d.Title) == "" || !plainText(d.Title, DiffTitleLimit) {
		return errors.New("a diff needs a title of at most 200 characters")
	}
	if !plainText(d.Detail, DiffDetailLimit) || !plainText(d.Path, DiffPathLimit) || !diffLanguage.MatchString(d.Language) {
		return errors.New("invalid diff detail, path or language")
	}
	if len(d.Before) > DiffSideLimit || len(d.After) > DiffSideLimit {
		return errors.New("a diff side exceeds 2 MiB")
	}
	if d.Mode != "" && d.Mode != "inline" && d.Mode != "split" {
		return errors.New("diff mode must be inline or split")
	}
	if len(d.Actions) > DiffActionsLimit {
		return errors.New("a diff may declare at most 20 actions")
	}
	seen := map[string]bool{}
	for _, a := range d.Actions {
		if !diffActionID.MatchString(a.ID) || seen[a.ID] {
			return fmt.Errorf("invalid or repeated diff action %q", a.ID)
		}
		seen[a.ID] = true
		if strings.TrimSpace(a.Label) == "" || !plainText(a.Label, 100) || !diffGlyph.MatchString(a.Glyph) {
			return fmt.Errorf("diff action %s needs a label of at most 100 characters", a.ID)
		}
		if a.Scope != "file" && a.Scope != "hunk" && a.Scope != "line" {
			return fmt.Errorf("diff action %s must apply to a file, hunk or line", a.ID)
		}
	}
	return nil
}
