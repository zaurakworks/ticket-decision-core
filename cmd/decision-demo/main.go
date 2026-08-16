package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ticket-decision-core/decision"
)

func main() {
	policy := mustLoad[decision.RuleSet]("demo/policy.json")
	before := mustLoad[decision.Input]("demo/round-1-ticket.json")
	after := mustLoad[decision.Input]("demo/round-2-collected.json")

	fmt.Println("ROUND 1: decision core requests fixed external collectors")
	printDecision(mustEvaluate(policy, before))
	fmt.Println("ROUND 2: external collectors returned facts; decision core evaluates again")
	printDecision(mustEvaluate(policy, after))
}

func mustLoad[T any](path string) T {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	var value T
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		panic(err)
	}
	return value
}

func mustEvaluate(policy decision.RuleSet, input decision.Input) decision.Decision {
	result, err := decision.Evaluate(policy, input)
	if err != nil {
		panic(err)
	}
	return result
}

func printDecision(value decision.Decision) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		panic(err)
	}
}
