package pii

import (
	"fmt"
	"strings"
)

type Mode uint8

const (
	ModeOff Mode = iota
	ModeCustomers
	ModeAll
)

func (m Mode) String() string {
	switch m {
	case ModeOff:
		return "off"
	case ModeCustomers:
		return "customers"
	case ModeAll:
		return "all"
	default:
		return "invalid"
	}
}

// ParseMode converts configured text into a validated policy mode.
func ParseMode(v string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "off":
		return ModeOff, nil
	case "customers":
		return ModeCustomers, nil
	case "all":
		return ModeAll, nil
	default:
		return ModeOff, fmt.Errorf("invalid PII mode %q (expected off|customers|all)", v)
	}
}

func IsEnabled(mode Mode) bool {
	return mode == ModeCustomers || mode == ModeAll
}

// EffectiveMode applies per-request override policy.
func EffectiveMode(configured string, allowUnredacted bool, unredacted bool) (Mode, error) {
	mode, err := ParseMode(configured)
	if err != nil {
		return ModeOff, err
	}
	if !unredacted {
		return mode, nil
	}
	// No redaction configured: override is effectively a no-op.
	if mode == ModeOff {
		return ModeOff, nil
	}
	if !allowUnredacted {
		return ModeOff, fmt.Errorf("--unredacted is disabled; set HS_INBOX_PII_ALLOW_UNREDACTED=1 or config inbox_pii_allow_unredacted: true to allow per-request overrides")
	}
	return ModeOff, nil
}

// ShouldRedactType decides whether an entity type should be redacted for mode.
// entityType is expected to be "customer" or "user". Unknown types are only
// redacted in "all" mode.
func ShouldRedactType(mode Mode, entityType string) bool {
	switch mode {
	case ModeAll:
		return true
	case ModeCustomers:
		return strings.EqualFold(strings.TrimSpace(entityType), "customer")
	default:
		return false
	}
}
