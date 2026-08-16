package decision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDemoPlansLogAPIAndDatabaseCollection(t *testing.T) {
	rules := loadFixture[RuleSet](t, filepath.Join("..", "demo", "policy.json"))
	input := loadFixture[Input](t, filepath.Join("..", "demo", "round-1-ticket.json"))

	got, err := Evaluate(rules, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNeedMoreInfo {
		t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeNeedMoreInfo)
	}
	if len(got.Collection) != 3 {
		t.Fatalf("collection requests = %d, want 3: %+v", len(got.Collection), got.Collection)
	}
	kinds := []CollectorKind{got.Collection[0].Kind, got.Collection[1].Kind, got.Collection[2].Kind}
	if !reflect.DeepEqual(kinds, []CollectorKind{CollectorLog, CollectorAPI, CollectorDatabase}) {
		t.Fatalf("collector kinds = %v", kinds)
	}
	if got.Collection[0].Parameters["request_id"] != "req-7f91" {
		t.Fatalf("log request parameters = %+v", got.Collection[0].Parameters)
	}
	if got.Collection[1].Parameters["order_id"] != "order-2048" || got.Collection[2].Parameters["order_id"] != "order-2048" {
		t.Fatalf("order parameters were not bound: %+v", got.Collection)
	}
	if len(got.UnresolvedFacts) != 0 {
		t.Fatalf("unresolved facts = %v", got.UnresolvedFacts)
	}
}

func TestDemoDecidesAfterExternalCollection(t *testing.T) {
	rules := loadFixture[RuleSet](t, filepath.Join("..", "demo", "policy.json"))
	input := loadFixture[Input](t, filepath.Join("..", "demo", "round-2-collected.json"))

	got, err := Evaluate(rules, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeActionable || got.ReasonCode != "stuck_order_has_pending_job" {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if len(got.Collection) != 0 || !reflect.DeepEqual(got.MatchedRuleIDs, []string{"retry-stuck-order"}) {
		t.Fatalf("unexpected decision details: %+v", got)
	}
}

func TestCollectorWithMissingParameterIsNotExecutable(t *testing.T) {
	collectors := []Collector{{
		ID: "logs", Kind: CollectorLog, Provides: []string{"evidence.logs"},
		Instruction: "logs.search", Parameters: map[string]string{"request_id": "ticket.request_id"},
	}}
	requests, unresolved := planCollections(collectors, []string{"evidence.logs"}, map[string]any{})
	if len(requests) != 0 {
		t.Fatalf("requests = %+v", requests)
	}
	if !reflect.DeepEqual(unresolved, []string{"ticket.request_id"}) {
		t.Fatalf("unresolved = %v", unresolved)
	}
}

func TestValidateRejectsMultipleCollectorsForOneFact(t *testing.T) {
	rules := accessRules()
	rules.Collectors = []Collector{
		{ID: "one", Kind: CollectorLog, Provides: []string{"evidence.x"}, Instruction: "logs.one"},
		{ID: "two", Kind: CollectorAPI, Provides: []string{"evidence.x"}, Instruction: "api.two"},
	}
	if err := rules.Validate(); err == nil {
		t.Fatal("expected duplicate provider validation error")
	}
}

func loadFixture[T any](t *testing.T, path string) T {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(content, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
