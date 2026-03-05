package pii

import "testing"

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ModeOff},
		{in: "off", want: ModeOff},
		{in: "customers", want: ModeCustomers},
		{in: "all", want: ModeAll},
		{in: "unknown", want: ModeOff},
	}
	for _, tt := range tests {
		if got := NormalizeMode(tt.in); got != tt.want {
			t.Fatalf("NormalizeMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsValidMode(t *testing.T) {
	valid := []string{"off", "customers", "all"}
	for _, v := range valid {
		if !IsValidMode(v) {
			t.Fatalf("IsValidMode(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"foo", "none", "partial"} {
		if IsValidMode(v) {
			t.Fatalf("IsValidMode(%q) = true, want false", v)
		}
	}
	// Case+whitespace normalization means these ARE valid
	for _, v := range []string{"ALL", "Customers", " all ", " OFF"} {
		if !IsValidMode(v) {
			t.Fatalf("IsValidMode(%q) = false, want true (normalized)", v)
		}
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"off", false},
		{"", false},
		{"customers", true},
		{"all", true},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := IsEnabled(tt.mode); got != tt.want {
			t.Fatalf("IsEnabled(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestShouldRedactType(t *testing.T) {
	tests := []struct {
		mode, entity string
		want         bool
	}{
		{"off", "customer", false},
		{"off", "user", false},
		{"customers", "customer", true},
		{"customers", "Customer", true}, // case-insensitive
		{"customers", "user", false},
		{"customers", "unknown", false},
		{"customers", "", false},
		{"all", "customer", true},
		{"all", "user", true},
		{"all", "unknown", true},
		{"all", "", true},
	}
	for _, tt := range tests {
		if got := ShouldRedactType(tt.mode, tt.entity); got != tt.want {
			t.Fatalf("ShouldRedactType(%q, %q) = %v, want %v", tt.mode, tt.entity, got, tt.want)
		}
	}
}

func TestEffectiveMode(t *testing.T) {
	mode, err := EffectiveMode(ModeCustomers, false, false)
	if err != nil || mode != ModeCustomers {
		t.Fatalf("expected customers mode, got mode=%q err=%v", mode, err)
	}

	mode, err = EffectiveMode(ModeCustomers, true, true)
	if err != nil || mode != ModeOff {
		t.Fatalf("expected off override, got mode=%q err=%v", mode, err)
	}

	_, err = EffectiveMode(ModeCustomers, false, true)
	if err == nil {
		t.Fatalf("expected error when override disallowed")
	}

	mode, err = EffectiveMode(ModeOff, false, true)
	if err != nil || mode != ModeOff {
		t.Fatalf("expected off no-op override, got mode=%q err=%v", mode, err)
	}
}

