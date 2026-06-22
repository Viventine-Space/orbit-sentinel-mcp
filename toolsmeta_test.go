package main

import (
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite tools.json from the registrations")

// TestToolsJSON keeps tools.json in lockstep with the wrapAddTool registrations.
// It fails in CI if the committed tools.json is stale; run with -update (or
// `go generate`) to regenerate it.
func TestToolsJSON(t *testing.T) {
	tools, err := parseRegisteredTools("tools.go")
	if err != nil {
		t.Fatalf("parse registrations: %v", err)
	}
	if len(tools) < 15 {
		t.Fatalf("parsed only %d tools — AST extraction is likely broken", len(tools))
	}

	got, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	if *update {
		if err := os.WriteFile("tools.json", got, 0o644); err != nil {
			t.Fatalf("write tools.json: %v", err)
		}
		return
	}

	want, err := os.ReadFile("tools.json")
	if err != nil {
		t.Fatalf("read tools.json: %v (run: go test -run TestToolsJSON -update)", err)
	}
	if string(got) != string(want) {
		t.Errorf("tools.json is stale vs the wrapAddTool registrations — run: go test -run TestToolsJSON -update")
	}
}
