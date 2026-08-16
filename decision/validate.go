package decision

import (
	"fmt"
	"strings"
)

var supportedOperators = map[string]struct{}{
	"eq": {}, "neq": {}, "in": {}, "not_in": {},
	"gt": {}, "gte": {}, "lt": {}, "lte": {},
	"contains": {}, "exists": {},
}

var authoredOutcomes = map[Outcome]struct{}{
	OutcomeActionable: {}, OutcomePolicyDenied: {},
	OutcomeAlreadySatisfied: {}, OutcomeManualReview: {},
}

var supportedCollectorKinds = map[CollectorKind]struct{}{
	CollectorLog: {}, CollectorAPI: {}, CollectorDatabase: {},
}

func (rs RuleSet) Validate() error {
	if rs.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	if strings.TrimSpace(rs.ID) == "" {
		return fmt.Errorf("ruleset id is required")
	}
	if strings.TrimSpace(rs.Version) == "" {
		return fmt.Errorf("ruleset version is required")
	}
	if len(rs.Rules) == 0 {
		return fmt.Errorf("ruleset must contain at least one rule")
	}

	collectorIDs := make(map[string]struct{}, len(rs.Collectors))
	providers := make(map[string]string)
	for i, collector := range rs.Collectors {
		path := fmt.Sprintf("collectors[%d]", i)
		if strings.TrimSpace(collector.ID) == "" {
			return fmt.Errorf("%s.id is required", path)
		}
		if _, exists := collectorIDs[collector.ID]; exists {
			return fmt.Errorf("duplicate collector id %q", collector.ID)
		}
		collectorIDs[collector.ID] = struct{}{}
		if _, ok := supportedCollectorKinds[collector.Kind]; !ok {
			return fmt.Errorf("%s.kind %q is unsupported", path, collector.Kind)
		}
		if strings.TrimSpace(collector.Instruction) == "" {
			return fmt.Errorf("%s.instruction is required", path)
		}
		if len(collector.Provides) == 0 {
			return fmt.Errorf("%s.provides must not be empty", path)
		}
		for j, fact := range collector.Provides {
			if strings.TrimSpace(fact) == "" {
				return fmt.Errorf("%s.provides[%d] must not be empty", path, j)
			}
			if existing, exists := providers[fact]; exists {
				return fmt.Errorf("fact %q has multiple collectors %q and %q", fact, existing, collector.ID)
			}
			providers[fact] = collector.ID
		}
		for name, fact := range collector.Parameters {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("%s.parameters contains an empty name", path)
			}
			if strings.TrimSpace(fact) == "" {
				return fmt.Errorf("%s.parameters[%q] must reference a fact", path, name)
			}
		}
	}

	ids := make(map[string]struct{}, len(rs.Rules))
	for i, rule := range rs.Rules {
		path := fmt.Sprintf("rules[%d]", i)
		if strings.TrimSpace(rule.ID) == "" {
			return fmt.Errorf("%s.id is required", path)
		}
		if _, exists := ids[rule.ID]; exists {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		ids[rule.ID] = struct{}{}
		if err := validateCondition(rule.When, path+".when"); err != nil {
			return err
		}
		if _, ok := authoredOutcomes[rule.Then.Outcome]; !ok {
			return fmt.Errorf("%s.then.outcome %q is not authorable", path, rule.Then.Outcome)
		}
		if strings.TrimSpace(rule.Then.ReasonCode) == "" {
			return fmt.Errorf("%s.then.reason_code is required", path)
		}
	}
	return nil
}

func (in Input) Validate() error {
	if strings.TrimSpace(in.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if in.Facts == nil {
		return fmt.Errorf("facts object is required")
	}
	return nil
}

func validateCondition(c Condition, path string) error {
	forms := 0
	if c.All != nil {
		forms++
	}
	if c.Any != nil {
		forms++
	}
	if c.Not != nil {
		forms++
	}
	if c.Fact != "" || c.Operator != "" {
		forms++
	}
	if forms != 1 {
		return fmt.Errorf("%s must define exactly one of all, any, not, or fact/operator", path)
	}

	switch {
	case c.All != nil:
		if len(c.All) == 0 {
			return fmt.Errorf("%s.all must not be empty", path)
		}
		for i, child := range c.All {
			if err := validateCondition(child, fmt.Sprintf("%s.all[%d]", path, i)); err != nil {
				return err
			}
		}
	case c.Any != nil:
		if len(c.Any) == 0 {
			return fmt.Errorf("%s.any must not be empty", path)
		}
		for i, child := range c.Any {
			if err := validateCondition(child, fmt.Sprintf("%s.any[%d]", path, i)); err != nil {
				return err
			}
		}
	case c.Not != nil:
		return validateCondition(*c.Not, path+".not")
	default:
		if strings.TrimSpace(c.Fact) == "" {
			return fmt.Errorf("%s.fact is required", path)
		}
		if c.Value == nil {
			return fmt.Errorf("%s.value is required and must not be null", path)
		}
		if _, ok := supportedOperators[c.Operator]; !ok {
			return fmt.Errorf("%s.operator %q is unsupported", path, c.Operator)
		}
		if c.Operator == "exists" {
			if _, ok := c.Value.(bool); !ok {
				return fmt.Errorf("%s.value must be boolean for exists", path)
			}
		}
	}
	return nil
}
