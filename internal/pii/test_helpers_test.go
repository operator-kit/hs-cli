package pii

import "strings"

func mustTestEngine(mode Mode, rawSecret string, opts ...EngineOption) *Engine {
	var pseudonym PseudonymContext
	if IsEnabled(mode) {
		if strings.TrimSpace(rawSecret) == "" {
			rawSecret = "unit-test-only-secret"
		}
		pseudonym = mustTestPseudonym(rawSecret, "test-v2")
	}
	engine, err := NewEngine(mode, pseudonym, opts...)
	if err != nil {
		panic(err)
	}
	return engine
}

func mustTestPseudonym(rawSecret, keyID string) PseudonymContext {
	secret, err := NewSecretString(rawSecret)
	if err != nil {
		panic(err)
	}
	pseudonym, err := NewPseudonymContext(secret, keyID)
	if err != nil {
		panic(err)
	}
	return pseudonym
}
