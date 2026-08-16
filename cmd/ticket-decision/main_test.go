package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ticket-decision-core/decision"
)

func TestEvaluateCommandEmitsDecisionJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"evaluate",
		"--rules", filepath.Join("..", "..", "testdata", "rules.json"),
		"--input", filepath.Join("..", "..", "testdata", "input-actionable.json"),
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var got decision.Decision
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output = %s", err, stdout.String())
	}
	if got.Outcome != decision.OutcomeActionable || got.ReasonCode != "standard_access_allowed" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestValidateCommandRejectsUnknownFields(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "rules.json")
	content := `{"schema_version":1,"id":"x","version":"1","rules":[],"unexpected":true}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"validate", "--rules", path}, &stdout, &stderr)
	if code != exitInvalid {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error: %v; stderr = %s", err, stderr.String())
	}
	if envelope.Error.Code != "invalid_ruleset" {
		t.Fatalf("error code = %q", envelope.Error.Code)
	}
}
