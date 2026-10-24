package extension

import (
	"encoding/json"
	"testing"
)

func TestProcessBytesSurviveJSON(t *testing.T) {
	input := RunInput{Program: "git", Args: []string{"diff", "bad\xffname"}, Stdin: "patch\xff\x00"}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var copy RunInput
	if err = json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	if copy.Args[1] != input.Args[1] || copy.Stdin != input.Stdin {
		t.Fatal("process input bytes changed")
	}
	result := RunResult{Stdout: "raw\xff\x00", Stderr: "error\xfe"}
	data, _ = json.Marshal(result)
	var got RunResult
	if err = json.Unmarshal(data, &got); err != nil || got.Stdout != result.Stdout || got.Stderr != result.Stderr {
		t.Fatal("process output bytes changed", err)
	}
	terminal := TerminalInput{Program: "git", Args: input.Args, Cwd: "/fixture"}
	data, _ = json.Marshal(terminal)
	var term TerminalInput
	if err = json.Unmarshal(data, &term); err != nil || term.Args[1] != input.Args[1] {
		t.Fatal("terminal argument bytes changed", err)
	}
}
