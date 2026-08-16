package decision

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestEvaluateActionable(t *testing.T) {
	rules := accessRules()
	input := Input{RunID: "ticket-1/revision-2", Facts: map[string]any{
		"ticket": map[string]any{"type": "access", "risk": "low"},
		"user":   map[string]any{"active": true},
	}}

	got, err := Evaluate(rules, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeActionable || got.ReasonCode != "standard_access" {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if !reflect.DeepEqual(got.MatchedRuleIDs, []string{"grant-standard"}) {
		t.Fatalf("unexpected matched rules: %v", got.MatchedRuleIDs)
	}
	if string(got.Directive) != `{"handler":"grant"}` {
		t.Fatalf("unexpected directive: %s", got.Directive)
	}
}

func TestEvaluateRequestsFactForHigherPriorityRule(t *testing.T) {
	rules := accessRules()
	input := Input{RunID: "ticket-1/revision-1", Facts: map[string]any{
		"ticket": map[string]any{"type": "access"},
		"user":   map[string]any{"active": true},
	}}

	got, err := Evaluate(rules, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNeedMoreInfo {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeNeedMoreInfo)
	}
	if !reflect.DeepEqual(got.MissingFacts, []string{"ticket.risk"}) {
		t.Fatalf("missing facts = %v", got.MissingFacts)
	}
}

func TestEvaluateRequestsLowerPriorityFactAfterHigherRuleIsFalse(t *testing.T) {
	rules := accessRules()
	input := Input{RunID: "ticket-1/revision-2", Facts: map[string]any{
		"ticket": map[string]any{"type": "access", "risk": "low"},
	}}

	got, err := Evaluate(rules, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNeedMoreInfo {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeNeedMoreInfo)
	}
	if !reflect.DeepEqual(got.MissingFacts, []string{"user.active"}) {
		t.Fatalf("missing facts = %v", got.MissingFacts)
	}
}

func TestEvaluateNoMatchWhenFactsDisproveEveryRule(t *testing.T) {
	got, err := Evaluate(accessRules(), Input{RunID: "ticket-2/revision-1", Facts: map[string]any{
		"ticket": map[string]any{"type": "hardware"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNoMatch || got.ReasonCode != "no_rule_match" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestEvaluateReportsSamePriorityConflict(t *testing.T) {
	rules := RuleSet{SchemaVersion: 1, ID: "conflict", Version: "1", Rules: []Rule{
		matchingRule("z-rule", 10, OutcomeActionable),
		matchingRule("a-rule", 10, OutcomeManualReview),
	}}
	got, err := Evaluate(rules, Input{RunID: "ticket-3/revision-1", Facts: map[string]any{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeAmbiguous || got.ReasonCode != "conflicting_rules" {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if !reflect.DeepEqual(got.MatchedRuleIDs, []string{"a-rule", "z-rule"}) {
		t.Fatalf("matched rules = %v", got.MatchedRuleIDs)
	}
}

func TestEvaluateLowerPriorityUnknownDoesNotBlockMatch(t *testing.T) {
	rules := RuleSet{SchemaVersion: 1, ID: "priority", Version: "1", Rules: []Rule{
		matchingRule("winner", 20, OutcomeActionable),
		{
			ID: "blocked-lower", Priority: 10,
			When: Condition{Fact: "not.collected", Operator: "eq", Value: true},
			Then: Result{Outcome: OutcomeManualReview, ReasonCode: "lower"},
		},
	}}
	got, err := Evaluate(rules, Input{RunID: "ticket-4/revision-1", Facts: map[string]any{"kind": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeActionable || !reflect.DeepEqual(got.MatchedRuleIDs, []string{"winner"}) {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestEvaluateRejectsComparisonTypeMismatch(t *testing.T) {
	rules := RuleSet{SchemaVersion: 1, ID: "types", Version: "1", Rules: []Rule{{
		ID: "numeric", Priority: 1,
		When: Condition{Fact: "age", Operator: "gte", Value: float64(18)},
		Then: Result{Outcome: OutcomeActionable, ReasonCode: "adult"},
	}}}
	_, err := Evaluate(rules, Input{RunID: "ticket-5/revision-1", Facts: map[string]any{"age": "unknown"}})
	if err == nil || !strings.Contains(err.Error(), "expects a numeric fact") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsEngineOwnedOutcome(t *testing.T) {
	rule := matchingRule("invalid", 1, OutcomeNeedMoreInfo)
	rules := RuleSet{SchemaVersion: 1, ID: "invalid", Version: "1", Rules: []Rule{rule}}
	if err := rules.Validate(); err == nil || !strings.Contains(err.Error(), "not authorable") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsMissingConditionValue(t *testing.T) {
	rule := matchingRule("invalid", 1, OutcomeActionable)
	rule.When.Value = nil
	rules := RuleSet{SchemaVersion: 1, ID: "invalid", Version: "1", Rules: []Rule{rule}}
	if err := rules.Validate(); err == nil || !strings.Contains(err.Error(), "value is required") {
		t.Fatalf("error = %v", err)
	}
}

func accessRules() RuleSet {
	return RuleSet{SchemaVersion: 1, ID: "access", Version: "1", Rules: []Rule{
		{
			ID: "deny-high-risk", Priority: 200,
			When: Condition{All: []Condition{
				{Fact: "ticket.type", Operator: "eq", Value: "access"},
				{Fact: "ticket.risk", Operator: "eq", Value: "high"},
			}},
			Then: Result{Outcome: OutcomePolicyDenied, ReasonCode: "high_risk"},
		},
		{
			ID: "grant-standard", Priority: 100,
			When: Condition{All: []Condition{
				{Fact: "ticket.type", Operator: "eq", Value: "access"},
				{Fact: "user.active", Operator: "eq", Value: true},
			}},
			Then: Result{Outcome: OutcomeActionable, ReasonCode: "standard_access", Directive: json.RawMessage(`{"handler":"grant"}`)},
		},
	}}
}

func matchingRule(id string, priority int, outcome Outcome) Rule {
	return Rule{
		ID: id, Priority: priority,
		When: Condition{Fact: "kind", Operator: "eq", Value: "test"},
		Then: Result{Outcome: outcome, ReasonCode: id},
	}
}
