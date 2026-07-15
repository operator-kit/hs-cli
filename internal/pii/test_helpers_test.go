package pii

import "strings"

func mustTestEngine(mode Mode, rawSecret string, opts ...EngineOption) *Engine {
	var secret Secret
	if IsEnabled(mode) {
		if strings.TrimSpace(rawSecret) == "" {
			rawSecret = "unit-test-only-secret"
		}
		var err error
		secret, err = NewSecretString(rawSecret)
		if err != nil {
			panic(err)
		}
	}
	engine, err := NewEngine(mode, secret, opts...)
	if err != nil {
		panic(err)
	}
	return engine
}
