package pii

import (
	"fmt"
	"strings"
)

const (
	ModeOff       = "off"
	ModeCustomers = "customers"
	ModeAll       = "all"
)

// NormalizeMode normalizes configured mode values.
func NormalizeMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", ModeOff:
		return ModeOff
	case ModeCustomers:
		return ModeCustomers
	case ModeAll:
		return ModeAll
	default:
		return ModeOff
	}
}

func IsValidMode(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ModeOff, ModeCustomers, ModeAll:
		return true
	default:
		return false
	}
}

func IsEnabled(mode string) bool {
	return NormalizeMode(mode) != ModeOff
}

// EffectiveMode applies per-request override policy.
func EffectiveMode(mode string, allowUnredacted bool, unredacted bool) (string, error) {
	normalized := NormalizeMode(mode)
	if !unredacted {
		return normalized, nil
	}
	// No redaction configured: override is effectively a no-op.
	if normalized == ModeOff {
		return ModeOff, nil
	}
	if !allowUnredacted {
		return "", fmt.Errorf("--unredacted is disabled; set HS_INBOX_PII_ALLOW_UNREDACTED=1 or config inbox_pii_allow_unredacted: true to allow per-request overrides")
	}
	return ModeOff, nil
}

// ShouldRedactType decides whether an entity type should be redacted for mode.
// entityType is expected to be "customer" or "user". Unknown types are only
// redacted in "all" mode.
func ShouldRedactType(mode, entityType string) bool {
	switch NormalizeMode(mode) {
	case ModeAll:
		return true
	case ModeCustomers:
		return strings.EqualFold(strings.TrimSpace(entityType), "customer")
	default:
		return false
	}
}

