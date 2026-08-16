package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"ticket-decision-core/decision"
)

const (
	exitOK      = 0
	exitUsage   = 2
	exitInvalid = 3
	exitFailure = 4
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeError(stderr, "usage", "expected validate or evaluate subcommand")
		return exitUsage
	}

	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "evaluate":
		return runEvaluate(args[1:], stdout, stderr)
	default:
		writeError(stderr, "usage", fmt.Sprintf("unknown subcommand %q", args[0]))
		return exitUsage
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rulesPath := flags.String("rules", "", "path to ruleset JSON")
	if err := flags.Parse(args); err != nil || *rulesPath == "" || flags.NArg() != 0 {
		writeError(stderr, "usage", "usage: ticket-decision validate --rules <rules.json>")
		return exitUsage
	}

	rules, err := loadJSON[decision.RuleSet](*rulesPath)
	if err != nil {
		writeError(stderr, "invalid_ruleset", err.Error())
		return exitInvalid
	}
	if err := rules.Validate(); err != nil {
		writeError(stderr, "invalid_ruleset", err.Error())
		return exitInvalid
	}

	if err := writeJSON(stdout, map[string]any{
		"valid": true, "ruleset_id": rules.ID, "ruleset_version": rules.Version,
	}); err != nil {
		writeError(stderr, "output_failed", err.Error())
		return exitFailure
	}
	return exitOK
}

func runEvaluate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rulesPath := flags.String("rules", "", "path to ruleset JSON")
	inputPath := flags.String("input", "", "path to input JSON")
	if err := flags.Parse(args); err != nil || *rulesPath == "" || *inputPath == "" || flags.NArg() != 0 {
		writeError(stderr, "usage", "usage: ticket-decision evaluate --rules <rules.json> --input <input.json>")
		return exitUsage
	}

	rules, err := loadJSON[decision.RuleSet](*rulesPath)
	if err != nil {
		writeError(stderr, "invalid_ruleset", err.Error())
		return exitInvalid
	}
	input, err := loadJSON[decision.Input](*inputPath)
	if err != nil {
		writeError(stderr, "invalid_input", err.Error())
		return exitInvalid
	}
	result, err := decision.Evaluate(rules, input)
	if err != nil {
		writeError(stderr, "evaluation_failed", err.Error())
		return exitInvalid
	}
	if err := writeJSON(stdout, result); err != nil {
		writeError(stderr, "output_failed", err.Error())
		return exitFailure
	}
	return exitOK
}

func loadJSON[T any](path string) (T, error) {
	var result T
	file, err := os.Open(path)
	if err != nil {
		return result, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return result, fmt.Errorf("decode %q: multiple JSON values", path)
		}
		return result, fmt.Errorf("decode %q: %w", path, err)
	}
	return result, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeError(writer io.Writer, code, message string) {
	_ = writeJSON(writer, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
