package selfupdate

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseVersion parses a semver string (with optional "v" prefix) into [major, minor, patch].
func ParseVersion(v string) ([3]int, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("invalid version: %q", v)
	}
	var ver [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("invalid version component %q: %w", p, err)
		}
		ver[i] = n
	}
	return ver, nil
}

// CompareVersions returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	va, err := ParseVersion(a)
	if err != nil {
		return 0
	}
	vb, err := ParseVersion(b)
	if err != nil {
		return 0
	}
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	return 0
}
