package decision

import (
	"reflect"
	"testing"
)

func TestAllIsFalseWhenOneBranchIsFalseAndAnotherMissing(t *testing.T) {
	condition := Condition{All: []Condition{
		{Fact: "missing", Operator: "eq", Value: true},
		{Fact: "known", Operator: "eq", Value: false},
	}}
	got, err := evaluateCondition(condition, map[string]any{"known": true})
	if err != nil {
		t.Fatal(err)
	}
	if got.truth != truthFalse || len(got.missing) != 0 {
		t.Fatalf("result = %+v", got)
	}
}

func TestAnyIsTrueWhenOneBranchMatchesAndAnotherIsMissing(t *testing.T) {
	condition := Condition{Any: []Condition{
		{Fact: "missing", Operator: "eq", Value: true},
		{Fact: "known", Operator: "eq", Value: true},
	}}
	got, err := evaluateCondition(condition, map[string]any{"known": true})
	if err != nil {
		t.Fatal(err)
	}
	if got.truth != truthTrue || len(got.missing) != 0 {
		t.Fatalf("result = %+v", got)
	}
}

func TestExistsMakesAbsenceDecidable(t *testing.T) {
	condition := Condition{Fact: "optional.value", Operator: "exists", Value: false}
	got, err := evaluateCondition(condition, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got.truth != truthTrue {
		t.Fatalf("result = %+v", got)
	}
}

func TestUnknownFactsAreUniqueAndSorted(t *testing.T) {
	condition := Condition{All: []Condition{
		{Fact: "z", Operator: "eq", Value: true},
		{Fact: "a", Operator: "eq", Value: true},
		{Fact: "z", Operator: "neq", Value: false},
	}}
	got, err := evaluateCondition(condition, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got.truth != truthUnknown || !reflect.DeepEqual(got.missing, []string{"a", "z"}) {
		t.Fatalf("result = %+v", got)
	}
}
