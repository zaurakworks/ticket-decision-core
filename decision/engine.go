package decision

import (
	"fmt"
	"sort"
)

type evaluatedRule struct {
	rule   Rule
	result conditionResult
}

// Evaluate deterministically maps a fact snapshot to one decision without performing I/O.
func Evaluate(rules RuleSet, input Input) (Decision, error) {
	if err := rules.Validate(); err != nil {
		return Decision{}, fmt.Errorf("invalid ruleset: %w", err)
	}
	if err := input.Validate(); err != nil {
		return Decision{}, fmt.Errorf("invalid input: %w", err)
	}

	base := Decision{
		RunID:          input.RunID,
		RuleSetID:      rules.ID,
		RuleSetVersion: rules.Version,
	}
	matched := make([]evaluatedRule, 0)
	blocked := make([]evaluatedRule, 0)

	for _, rule := range rules.Rules {
		result, err := evaluateCondition(rule.When, input.Facts)
		if err != nil {
			return Decision{}, fmt.Errorf("evaluate rule %q: %w", rule.ID, err)
		}
		switch result.truth {
		case truthTrue:
			matched = append(matched, evaluatedRule{rule: rule, result: result})
		case truthUnknown:
			blocked = append(blocked, evaluatedRule{rule: rule, result: result})
		}
	}

	bestMatchedPriority, hasMatch := highestPriority(matched)
	bestBlockedPriority, hasBlocked := highestPriority(blocked)

	// A rule that could tie or outrank the current match must be resolved first.
	if hasBlocked && (!hasMatch || bestBlockedPriority >= bestMatchedPriority) {
		missing := make([]string, 0)
		for _, candidate := range blocked {
			if candidate.rule.Priority == bestBlockedPriority {
				missing = append(missing, candidate.result.missing...)
			}
		}
		base.Outcome = OutcomeNeedMoreInfo
		base.ReasonCode = "missing_facts"
		base.MissingFacts = uniqueSorted(missing)
		base.Collection, base.UnresolvedFacts = planCollections(rules.Collectors, base.MissingFacts, input.Facts)
		return base, nil
	}

	if !hasMatch {
		base.Outcome = OutcomeNoMatch
		base.ReasonCode = "no_rule_match"
		return base, nil
	}

	winners := make([]Rule, 0, 1)
	for _, candidate := range matched {
		if candidate.rule.Priority == bestMatchedPriority {
			winners = append(winners, candidate.rule)
		}
	}
	sort.Slice(winners, func(i, j int) bool { return winners[i].ID < winners[j].ID })
	if len(winners) > 1 {
		base.Outcome = OutcomeAmbiguous
		base.ReasonCode = "conflicting_rules"
		base.MatchedRuleIDs = make([]string, len(winners))
		for i, winner := range winners {
			base.MatchedRuleIDs[i] = winner.ID
		}
		return base, nil
	}

	winner := winners[0]
	base.Outcome = winner.Then.Outcome
	base.ReasonCode = winner.Then.ReasonCode
	base.MatchedRuleIDs = []string{winner.ID}
	if len(winner.Then.Directive) > 0 {
		base.Directive = append(base.Directive, winner.Then.Directive...)
	}
	return base, nil
}

func highestPriority(candidates []evaluatedRule) (int, bool) {
	if len(candidates) == 0 {
		return 0, false
	}
	highest := candidates[0].rule.Priority
	for _, candidate := range candidates[1:] {
		if candidate.rule.Priority > highest {
			highest = candidate.rule.Priority
		}
	}
	return highest, true
}
