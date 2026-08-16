package decision

import "encoding/json"

// Outcome is a complete, machine-readable decision made without performing I/O.
type Outcome string

const (
	OutcomeActionable       Outcome = "actionable"
	OutcomeNeedMoreInfo     Outcome = "need_more_info"
	OutcomeNoMatch          Outcome = "no_match"
	OutcomeAmbiguous        Outcome = "ambiguous"
	OutcomePolicyDenied     Outcome = "policy_denied"
	OutcomeAlreadySatisfied Outcome = "already_satisfied"
	OutcomeManualReview     Outcome = "manual_review"
)

// CollectorKind identifies which external adapter executes a fixed collection instruction.
type CollectorKind string

const (
	CollectorLog      CollectorKind = "log"
	CollectorAPI      CollectorKind = "api"
	CollectorDatabase CollectorKind = "database"
)

// RuleSet is an immutable, versioned collection policy and prioritized rule collection.
type RuleSet struct {
	SchemaVersion int         `json:"schema_version"`
	ID            string      `json:"id"`
	Version       string      `json:"version"`
	Collectors    []Collector `json:"collectors,omitempty"`
	Rules         []Rule      `json:"rules"`
}

// Collector maps missing facts to a fixed external instruction and mutable query parameters.
// Parameter values are fact paths; executable commands, credentials, and connections stay external.
type Collector struct {
	ID          string            `json:"id"`
	Kind        CollectorKind     `json:"kind"`
	Provides    []string          `json:"provides"`
	Instruction string            `json:"instruction"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// CollectionRequest is a fully bound request for an external collector.
type CollectionRequest struct {
	CollectorID string         `json:"collector_id"`
	Kind        CollectorKind  `json:"kind"`
	Instruction string         `json:"instruction"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Produces    []string       `json:"produces"`
}

// Rule describes when a terminal decision is available. Higher Priority wins.
type Rule struct {
	ID       string    `json:"id"`
	Priority int       `json:"priority"`
	When     Condition `json:"when"`
	Then     Result    `json:"then"`
}

// Result is returned to the external collector/operator. Directive is opaque to this engine.
type Result struct {
	Outcome    Outcome         `json:"outcome"`
	ReasonCode string          `json:"reason_code"`
	Directive  json.RawMessage `json:"directive,omitempty"`
}

// Condition is exactly one of all, any, not, or a leaf fact comparison.
type Condition struct {
	All      []Condition `json:"all,omitempty"`
	Any      []Condition `json:"any,omitempty"`
	Not      *Condition  `json:"not,omitempty"`
	Fact     string      `json:"fact,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Value    any         `json:"value,omitempty"`
}

// Input is a fact snapshot supplied by an external collector.
type Input struct {
	RunID string         `json:"run_id"`
	Facts map[string]any `json:"facts"`
}

// Decision is the only output contract. The engine never collects data or executes directives.
type Decision struct {
	RunID           string              `json:"run_id"`
	RuleSetID       string              `json:"ruleset_id"`
	RuleSetVersion  string              `json:"ruleset_version"`
	Outcome         Outcome             `json:"outcome"`
	ReasonCode      string              `json:"reason_code"`
	MatchedRuleIDs  []string            `json:"matched_rule_ids,omitempty"`
	MissingFacts    []string            `json:"missing_facts,omitempty"`
	Collection      []CollectionRequest `json:"collection_requests,omitempty"`
	UnresolvedFacts []string            `json:"unresolved_facts,omitempty"`
	Directive       json.RawMessage     `json:"directive,omitempty"`
}
