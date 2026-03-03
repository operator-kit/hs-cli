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

