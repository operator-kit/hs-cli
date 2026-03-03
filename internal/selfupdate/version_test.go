package selfupdate

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
		err   bool
	}{
		{"1.2.3", [3]int{1, 2, 3}, false},
		{"v1.2.3", [3]int{1, 2, 3}, false},
		{"0.1.0", [3]int{0, 1, 0}, false},
		{"v10.20.30", [3]int{10, 20, 30}, false},
		{"1.2", [3]int{}, true},
		{"abc", [3]int{}, true},
		{"1.2.x", [3]int{}, true},
		{"", [3]int{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.9.9", "1.0.0", -1},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"invalid", "1.0.0", 0},
		{"1.0.0", "invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
