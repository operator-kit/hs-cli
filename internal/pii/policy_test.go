package pii

import (
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Mode
	}{
		{name: "empty defaults off", in: "", want: ModeOff},
		{name: "off", in: "off", want: ModeOff},
		{name: "customers", in: "customers", want: ModeCustomers},
		{name: "all", in: "all", want: ModeAll},
		{name: "case and whitespace", in: "  Customers ", want: ModeCustomers},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.in)
			if err != nil {
				t.Fatalf("ParseMode(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseModeRejectsUnknownValues(t *testing.T) {
	for _, raw := range []string{"unknown", "none", "partial"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseMode(raw)
			if err == nil {
				t.Fatalf("ParseMode(%q) returned no error", raw)
			}
			for _, want := range []string{raw, "off", "customers", "all"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("ParseMode(%q) error %q does not contain %q", raw, err, want)
				}
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		mode Mode
		want bool
	}{
		{ModeOff, false},
		{ModeCustomers, true},
		{ModeAll, true},
	}
	for _, tt := range tests {
		if got := IsEnabled(tt.mode); got != tt.want {
			t.Fatalf("IsEnabled(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestShouldRedactType(t *testing.T) {
	tests := []struct {
		mode   Mode
		entity string
		want   bool
	}{
		{ModeOff, "customer", false},
		{ModeOff, "user", false},
		{ModeCustomers, "customer", true},
		{ModeCustomers, "Customer", true}, // case-insensitive
		{ModeCustomers, "user", false},
		{ModeCustomers, "unknown", false},
		{ModeCustomers, "", false},
		{ModeAll, "customer", true},
		{ModeAll, "user", true},
		{ModeAll, "unknown", true},
		{ModeAll, "", true},
	}
	for _, tt := range tests {
		if got := ShouldRedactType(tt.mode, tt.entity); got != tt.want {
			t.Fatalf("ShouldRedactType(%q, %q) = %v, want %v", tt.mode, tt.entity, got, tt.want)
		}
	}
}

func TestEffectiveMode(t *testing.T) {
	mode, err := EffectiveMode("customers", false, false)
	if err != nil || mode != ModeCustomers {
		t.Fatalf("expected customers mode, got mode=%q err=%v", mode, err)
	}

	mode, err = EffectiveMode("customers", true, true)
	if err != nil || mode != ModeOff {
		t.Fatalf("expected off override, got mode=%q err=%v", mode, err)
	}

	_, err = EffectiveMode("customers", false, true)
	if err == nil {
		t.Fatalf("expected error when override disallowed")
	}

	mode, err = EffectiveMode("off", false, true)
	if err != nil || mode != ModeOff {
		t.Fatalf("expected off no-op override, got mode=%q err=%v", mode, err)
	}
}

func TestEffectiveModeRejectsInvalidConfiguredMode(t *testing.T) {
	if _, err := EffectiveMode("typo", true, true); err == nil {
		t.Fatal("invalid configured mode must fail before applying an override")
	}
}
