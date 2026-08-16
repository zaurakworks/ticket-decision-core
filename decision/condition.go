package decision

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type truth uint8

const (
	truthFalse truth = iota
	truthTrue
	truthUnknown
)

type conditionResult struct {
	truth   truth
	missing []string
}

func evaluateCondition(c Condition, facts map[string]any) (conditionResult, error) {
	switch {
	case c.All != nil:
		missing := make([]string, 0)
		for _, child := range c.All {
			result, err := evaluateCondition(child, facts)
			if err != nil {
				return conditionResult{}, err
			}
			if result.truth == truthFalse {
				return conditionResult{truth: truthFalse}, nil
			}
			if result.truth == truthUnknown {
				missing = append(missing, result.missing...)
			}
		}
		if len(missing) > 0 {
			return conditionResult{truth: truthUnknown, missing: uniqueSorted(missing)}, nil
		}
		return conditionResult{truth: truthTrue}, nil

	case c.Any != nil:
		missing := make([]string, 0)
		for _, child := range c.Any {
			result, err := evaluateCondition(child, facts)
			if err != nil {
				return conditionResult{}, err
			}
			if result.truth == truthTrue {
				return conditionResult{truth: truthTrue}, nil
			}
			if result.truth == truthUnknown {
				missing = append(missing, result.missing...)
			}
		}
		if len(missing) > 0 {
			return conditionResult{truth: truthUnknown, missing: uniqueSorted(missing)}, nil
		}
		return conditionResult{truth: truthFalse}, nil

	case c.Not != nil:
		result, err := evaluateCondition(*c.Not, facts)
		if err != nil {
			return conditionResult{}, err
		}
		if result.truth == truthTrue {
			result.truth = truthFalse
		} else if result.truth == truthFalse {
			result.truth = truthTrue
		}
		return result, nil

	default:
		return evaluateLeaf(c, facts)
	}
}

func evaluateLeaf(c Condition, facts map[string]any) (conditionResult, error) {
	actual, present := lookupFact(facts, c.Fact)
	if c.Operator == "exists" {
		expected := c.Value.(bool)
		return boolResult(present == expected), nil
	}
	if !present {
		return conditionResult{truth: truthUnknown, missing: []string{c.Fact}}, nil
	}

	matched, err := compare(c.Operator, actual, c.Value)
	if err != nil {
		return conditionResult{}, fmt.Errorf("fact %q: %w", c.Fact, err)
	}
	return boolResult(matched), nil
}

func compare(operator string, actual, expected any) (bool, error) {
	switch operator {
	case "eq":
		return reflect.DeepEqual(actual, expected), nil
	case "neq":
		return !reflect.DeepEqual(actual, expected), nil
	case "in", "not_in":
		values, ok := expected.([]any)
		if !ok {
			return false, fmt.Errorf("%s expects an array value", operator)
		}
		found := false
		for _, value := range values {
			if reflect.DeepEqual(actual, value) {
				found = true
				break
			}
		}
		if operator == "not_in" {
			return !found, nil
		}
		return found, nil
	case "gt", "gte", "lt", "lte":
		left, ok := asNumber(actual)
		if !ok {
			return false, fmt.Errorf("%s expects a numeric fact", operator)
		}
		right, ok := asNumber(expected)
		if !ok {
			return false, fmt.Errorf("%s expects a numeric rule value", operator)
		}
		switch operator {
		case "gt":
			return left > right, nil
		case "gte":
			return left >= right, nil
		case "lt":
			return left < right, nil
		default:
			return left <= right, nil
		}
	case "contains":
		switch value := actual.(type) {
		case string:
			needle, ok := expected.(string)
			if !ok {
				return false, fmt.Errorf("contains on a string expects a string rule value")
			}
			return strings.Contains(value, needle), nil
		case []any:
			for _, item := range value {
				if reflect.DeepEqual(item, expected) {
					return true, nil
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("contains expects a string or array fact")
		}
	default:
		return false, fmt.Errorf("unsupported operator %q", operator)
	}
}

func lookupFact(facts map[string]any, path string) (any, bool) {
	var current any = facts
	for _, segment := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func asNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	default:
		return 0, false
	}
}

func boolResult(value bool) conditionResult {
	if value {
		return conditionResult{truth: truthTrue}
	}
	return conditionResult{truth: truthFalse}
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
