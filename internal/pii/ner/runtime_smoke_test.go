package ner

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRuntimeBundleSmoke(t *testing.T) {
	dir := os.Getenv("HS_PII_MODEL_SMOKE_DIR")
	if dir == "" {
		t.Skip("set HS_PII_MODEL_SMOKE_DIR to execute the real runtime/model smoke test")
	}
	platform := CurrentPlatform()
	if capability := RuntimeCapabilityFor(platform); !capability.Supported {
		t.Fatalf("workflow attempted smoke test on unsupported target %s", platform.Key())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := validateRuntimeBundle(ctx, pathsAt(dir, platform)); err != nil {
		t.Fatalf("runtime/model smoke validation: %v", err)
	}
}
