package ner

import (
	"fmt"
	"runtime"
)

type Platform struct {
	OS   string
	Arch string
}

func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

func (p Platform) Key() string {
	return p.OS + "-" + p.Arch
}

type RuntimeCapability struct {
	Platform  Platform
	Supported bool
	Reason    string
}

var supportedPlatforms = []Platform{
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
}

func SupportedPlatforms() []Platform {
	return append([]Platform(nil), supportedPlatforms...)
}

func RuntimeCapabilityFor(platform Platform) RuntimeCapability {
	for _, supported := range supportedPlatforms {
		if platform == supported {
			return RuntimeCapability{Platform: platform, Supported: true}
		}
	}
	return RuntimeCapability{
		Platform: platform,
		Reason: fmt.Sprintf(
			"local NER runtime is not supported on %s; free-form content remains hidden",
			platform.Key(),
		),
	}
}
