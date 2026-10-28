package extension

import (
	"encoding/json"
	"strings"
)

// OpenState is the navigation state Dinah gives a view when the person
// follows a link to a file of a type the view declares in View.Opens. The
// frame receives its JSON text as the state that dinah.ready resolves to, or
// in a state event when a window of the view is already open in the file's
// workspace. The browser bridge's dinah.openedFile(state) reads it.
type OpenState struct {
	Open OpenedFile `json:"open"`
}

// OpenedFile is the file Dinah asks a view to open. Path is its absolute
// path, which host.read accepts. MediaType is the declared type the file
// matched. Resource is the file in Dinah's workspace: the workspace the view
// opened in, the workspace source the file is in, and the file's path within
// that source with forward slashes. Line is the line the link asked for, or
// zero for none.
type OpenedFile struct {
	Path      string       `json:"path"`
	MediaType string       `json:"mediaType"`
	Resource  FileResource `json:"resource"`
	Line      int          `json:"line,omitempty"`
}

// FileResource is a file's place in one of Dinah's workspaces.
type FileResource struct {
	WorkspaceID string `json:"workspaceId"`
	SourceID    string `json:"sourceId"`
	Path        string `json:"path"`
}

// OpenStateLimit is the largest navigation state a frame may hold, which
// bounds an OpenState's JSON as well.
const OpenStateLimit = 8192

// ParseOpenState reads a view's navigation state as an OpenState. It
// reports false for any other state, so a view may keep states of its own
// that are not JSON objects with an "open" member.
func ParseOpenState(state string) (OpenedFile, bool) {
	if len(state) > OpenStateLimit || !strings.HasPrefix(strings.TrimSpace(state), "{") {
		return OpenedFile{}, false
	}
	var s OpenState
	if json.Unmarshal([]byte(state), &s) != nil || s.Open.Path == "" || s.Open.MediaType == "" {
		return OpenedFile{}, false
	}
	return s.Open, true
}
