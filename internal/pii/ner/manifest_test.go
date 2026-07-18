package ner

import (
	"slices"
	"strings"
	"testing"
)

func TestTrustedManifestCoversExactlySupportedPlatforms(t *testing.T) {
	manifest, err := LoadTrustedManifest()
	if err != nil {
		t.Fatalf("LoadTrustedManifest: %v", err)
	}
	got := platformKeys(bundlePlatforms(manifest.Bundles))
	want := platformKeys(SupportedPlatforms())
	if !slices.Equal(got, want) {
		t.Fatalf("manifest platforms = %v, want %v", got, want)
	}
	for _, platform := range SupportedPlatforms() {
		bundle, err := manifest.BundleFor(platform)
		if err != nil {
			t.Fatal(err)
		}
		if bundle.Archive.SHA256 == "" || bundle.Archive.Size <= 0 {
			t.Fatalf("%s has no trusted archive identity", platform.Key())
		}
	}
}

func TestTrustedManifestPinsPublishedReleaseDigests(t *testing.T) {
	manifest, err := LoadTrustedManifest()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"darwin-amd64": "6c42a556168d6f4251d34032b85a699a0e222c9ed14e1548f401525a0b51c5bd",
		"darwin-arm64": "01c7675685a779519d8cf1cb8c506f17bb46aab9abdb127f0813e7a01a6bbd4c",
		"linux-amd64":  "17aaf46aecb79ce9758f7fcca813894a2a9d24e97976bcf1b91ecf93c9f6dbfb",
		"linux-arm64":  "a50e5abd1e665e2fae8476b741d11c90b09ef6259d7dae2a42bc7fd78b980cd9",
	}
	for _, bundle := range manifest.Bundles {
		if got := bundle.Archive.SHA256; got != want[bundle.Platform().Key()] {
			t.Fatalf("%s archive SHA-256 = %s, want %s", bundle.Platform().Key(), got, want[bundle.Platform().Key()])
		}
	}
}

func TestTrustedManifestRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	withUnknown := []byte(strings.Replace(
		string(embeddedTrustedManifest),
		"\"schema_version\": 1",
		"\"schema_version\": 1, \"surprise\": true",
		1,
	))
	if _, err := decodeTrustedManifest(withUnknown); err == nil {
		t.Fatal("manifest accepted an unknown field")
	}
	withTrailing := append(append([]byte(nil), embeddedTrustedManifest...), []byte(" {}")...)
	if _, err := decodeTrustedManifest(withTrailing); err == nil {
		t.Fatal("manifest accepted trailing JSON")
	}
}

func TestTrustedManifestRejectsDuplicateOrUnsupportedCoverage(t *testing.T) {
	manifest, err := LoadTrustedManifest()
	if err != nil {
		t.Fatal(err)
	}
	duplicate := manifest
	duplicate.Bundles = append([]BundleManifest(nil), manifest.Bundles...)
	duplicate.Bundles[len(duplicate.Bundles)-1].GOOS = duplicate.Bundles[0].GOOS
	duplicate.Bundles[len(duplicate.Bundles)-1].GOARCH = duplicate.Bundles[0].GOARCH
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate coverage error = %v", err)
	}

	unsupported := manifest
	unsupported.Bundles = append([]BundleManifest(nil), manifest.Bundles...)
	unsupported.Bundles[0].GOOS = "windows"
	if err := unsupported.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported coverage error = %v", err)
	}
}

func bundlePlatforms(bundles []BundleManifest) []Platform {
	platforms := make([]Platform, 0, len(bundles))
	for _, bundle := range bundles {
		platforms = append(platforms, bundle.Platform())
	}
	return platforms
}

func platformKeys(platforms []Platform) []string {
	keys := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		keys = append(keys, platform.Key())
	}
	slices.Sort(keys)
	return keys
}
