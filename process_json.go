package extension

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

// EncodedArguments preserves Unix filenames that are not UTF-8. The ordinary
// args array remains a readable display; argsBase64 is authoritative if present.
func EncodedArguments(args []string) []string {
	needed := false
	for _, arg := range args {
		needed = needed || !utf8.ValidString(arg)
	}
	if !needed {
		return nil
	}
	encoded := make([]string, len(args))
	for i, arg := range args {
		encoded[i] = base64.StdEncoding.EncodeToString([]byte(arg))
	}
	return encoded
}
func decodeArguments(display, encoded []string) ([]string, error) {
	if encoded == nil {
		return display, nil
	}
	if len(encoded) > 256 {
		return nil, errors.New("too many encoded arguments")
	}
	result := make([]string, len(encoded))
	for i, arg := range encoded {
		raw, err := base64.StdEncoding.DecodeString(arg)
		if err != nil {
			return nil, err
		}
		result[i] = string(raw)
	}
	return result, nil
}
func encodeBytes(value string) string {
	if utf8.ValidString(value) {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(value))
}
func decodeBytes(display, encoded string) (string, error) {
	if encoded == "" {
		return display, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	return string(data), err
}

func (v RunInput) MarshalJSON() ([]byte, error) {
	type plain RunInput
	return json.Marshal(struct {
		plain
		ArgsBase64  []string `json:"argsBase64,omitempty"`
		StdinBase64 string   `json:"stdinBase64,omitempty"`
	}{plain(v), EncodedArguments(v.Args), encodeBytes(v.Stdin)})
}
func (v *RunInput) UnmarshalJSON(data []byte) error {
	type plain RunInput
	var wire struct {
		plain
		ArgsBase64  []string `json:"argsBase64"`
		StdinBase64 string   `json:"stdinBase64"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*v = RunInput(wire.plain)
	var err error
	if v.Args, err = decodeArguments(v.Args, wire.ArgsBase64); err != nil {
		return err
	}
	v.Stdin, err = decodeBytes(v.Stdin, wire.StdinBase64)
	return err
}
func (v RunResult) MarshalJSON() ([]byte, error) {
	type plain RunResult
	return json.Marshal(struct {
		plain
		StdoutBase64 string `json:"stdoutBase64,omitempty"`
		StderrBase64 string `json:"stderrBase64,omitempty"`
	}{plain(v), encodeBytes(v.Stdout), encodeBytes(v.Stderr)})
}
func (v *RunResult) UnmarshalJSON(data []byte) error {
	type plain RunResult
	var wire struct {
		plain
		StdoutBase64 string `json:"stdoutBase64"`
		StderrBase64 string `json:"stderrBase64"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*v = RunResult(wire.plain)
	var err error
	if v.Stdout, err = decodeBytes(v.Stdout, wire.StdoutBase64); err != nil {
		return err
	}
	v.Stderr, err = decodeBytes(v.Stderr, wire.StderrBase64)
	return err
}
func (v TerminalInput) MarshalJSON() ([]byte, error) {
	type plain TerminalInput
	return json.Marshal(struct {
		plain
		ArgsBase64 []string `json:"argsBase64,omitempty"`
	}{plain(v), EncodedArguments(v.Args)})
}
func (v *TerminalInput) UnmarshalJSON(data []byte) error {
	type plain TerminalInput
	var wire struct {
		plain
		ArgsBase64 []string `json:"argsBase64"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*v = TerminalInput(wire.plain)
	var err error
	v.Args, err = decodeArguments(v.Args, wire.ArgsBase64)
	return err
}
