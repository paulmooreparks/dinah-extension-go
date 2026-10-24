package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type Workspace struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Sources []Source `json:"sources"`
}
type Scope struct {
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	Workspace Workspace `json:"workspace"`
}
type Host struct {
	Scope  Scope
	Client *http.Client
}
type HostRequest struct {
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}
type HostResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
type RunInput struct {
	Program        string   `json:"program"`
	Args           []string `json:"args"`
	Cwd            string   `json:"cwd"`
	Stdin          string   `json:"stdin,omitempty"`
	Explanation    string   `json:"explanation"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}
type RunResult struct {
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	Truncated    bool   `json:"truncated"`
	Milliseconds int64  `json:"milliseconds"`
}
type FileInput struct {
	Path           string `json:"path"`
	Text           string `json:"text,omitempty"`
	ExpectedSHA256 string `json:"expectedSha256,omitempty"`
}
type FileResult struct {
	Text   string `json:"text"`
	SHA256 string `json:"sha256"`
}
type TerminalInput struct {
	Cwd     string   `json:"cwd"`
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

func (h *Host) Call(ctx context.Context, tool string, input, output any) error {
	u, err := url.Parse(h.Scope.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || h.Scope.Token == "" {
		return errors.New("invalid host scope")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	body, err := json.Marshal(HostRequest{Tool: tool, Input: data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.Scope.Token)
	req.Header.Set("Content-Type", "application/json")
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("host redirects are not allowed") }}
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var result HostResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(&result); err != nil {
		return fmt.Errorf("host %s response: %w", tool, err)
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("host %s returned HTTP %d", tool, response.StatusCode)
	}
	if output != nil {
		return json.Unmarshal(result.Result, output)
	}
	return nil
}
func (h *Host) Run(ctx context.Context, in RunInput) (RunResult, error) {
	var out RunResult
	err := h.Call(ctx, "host.run", in, &out)
	return out, err
}
