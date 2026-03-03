package permission

import (
	"fmt"
	"strings"
)

// Valid operations.
const (
	OpRead   = "read"
	OpWrite  = "write"
	OpDelete = "delete"
)

var validOps = map[string]bool{
	OpRead:   true,
	OpWrite:  true,
	OpDelete: true,
	"*":      true,
}

// Rule is a single resource:operation pair.
type Rule struct {
	Resource  string
	Operation string
}

// Policy is a parsed set of permission rules.
// A nil or zero-value Policy is unrestricted (allows everything).
type Policy struct {
	rules []Rule
	raw   string
}

// Parse parses a comma-separated permission string into a Policy.
// An empty string yields an unrestricted policy.
func Parse(s string) (*Policy, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &Policy{}, nil
	}

	parts := strings.Split(s, ",")
	rules := make([]Rule, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, ':')
		if idx < 0 {
			return nil, fmt.Errorf("invalid permission %q: expected resource:operation", part)
		}
		resource := part[:idx]
		op := part[idx+1:]
		if resource == "" {
			return nil, fmt.Errorf("invalid permission %q: empty resource", part)
		}
		if !validOps[op] {
			return nil, fmt.Errorf("invalid permission %q: operation must be read|write|delete|*", part)
		}
		rules = append(rules, Rule{Resource: resource, Operation: op})
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("empty permission string")
	}
	return &Policy{rules: rules, raw: s}, nil
}

// IsUnrestricted returns true when no rules are set (everything allowed).
func (p *Policy) IsUnrestricted() bool {
	return len(p.rules) == 0
}

// Allows checks whether the given resource:operation is permitted.
func (p *Policy) Allows(resource, op string) bool {
	if p.IsUnrestricted() {
		return true
	}
	for _, r := range p.rules {
		resMatch := r.Resource == "*" || r.Resource == resource
		opMatch := r.Operation == "*" || r.Operation == op
		if resMatch && opMatch {
			return true
		}
	}
	return false
}

// String returns the original permission string, or "unrestricted".
func (p *Policy) String() string {
	if p.IsUnrestricted() {
		return "unrestricted"
	}
	return p.raw
}

// Rules returns a copy of the parsed rules.
func (p *Policy) Rules() []Rule {
	out := make([]Rule, len(p.rules))
	copy(out, p.rules)
	return out
}
