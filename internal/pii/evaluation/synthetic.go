package evaluation

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	SyntheticSecretGenerator = "hs-privacy-structural"
	SyntheticSecretVersion   = 1
)

type SyntheticProvenance struct {
	Generator string `json:"generator"`
	Version   int    `json:"version"`
	Seed      string `json:"seed"`
	Recipe    string `json:"recipe"`
	Purpose   string `json:"purpose"`
}

var syntheticRecipes = func() map[string]bool {
	out := make(map[string]bool, len(RequiredSecretFamilies)+3)
	for _, family := range RequiredSecretFamilies {
		out[family] = true
	}
	out["checksum"] = true
	out["redacted-marker"] = true
	out["command-secret"] = true
	return out
}()

// GenerateSyntheticValue deterministically produces structurally realistic,
// offline-only fixture material. Provenance lives outside the generated text;
// validators recompute every declared value byte-for-byte.
func GenerateSyntheticValue(provenance SyntheticProvenance) (string, error) {
	if provenance.Generator != SyntheticSecretGenerator || provenance.Version != SyntheticSecretVersion {
		return "", fmt.Errorf("unsupported synthetic generator %q version %d", provenance.Generator, provenance.Version)
	}
	if !idPattern.MatchString(provenance.Seed) {
		return "", fmt.Errorf("invalid synthetic seed %q", provenance.Seed)
	}
	if !syntheticRecipes[provenance.Recipe] {
		return "", fmt.Errorf("unsupported synthetic recipe %q", provenance.Recipe)
	}
	if provenance.Purpose != "must-detect" && provenance.Purpose != "preserve" {
		return "", fmt.Errorf("unsupported synthetic purpose %q", provenance.Purpose)
	}
	if provenance.Purpose == "preserve" {
		return generateSyntheticNearMiss(provenance)
	}
	return generateSyntheticCredential(provenance)
}

func generateSyntheticCredential(provenance SyntheticProvenance) (string, error) {
	switch provenance.Recipe {
	case "api-key":
		return varySyntheticCredential(provenance, "ak_"+syntheticAlphabet(provenance, "api", upperAlphaNumeric, 32)), nil
	case "access-token":
		return varySyntheticCredential(provenance, "access_token_"+syntheticAlphabet(provenance, "access", alphaNumeric, 40)), nil
	case "oauth-token":
		return varySyntheticCredential(provenance, "oat2_"+syntheticAlphabet(provenance, "oauth", alphaNumericDash, 52)), nil
	case "password":
		return varySyntheticCredential(provenance, "R7!"+syntheticAlphabet(provenance, "password", alphaNumeric, 14)+"#q2"), nil
	case "jwt":
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"offline-fixture","aud":"privacy-eval"}`))
		signature := syntheticAlphabet(provenance, "jwt-signature", alphaNumericDash, 43)
		return varySyntheticCredential(provenance, header+"."+payload+"."+signature), nil
	case "one-time-code":
		switch syntheticVariation(provenance) % 5 {
		case 0:
			return syntheticDigits(provenance, "otp", 6), nil
		case 1:
			return syntheticDigits(provenance, "otp", 8), nil
		case 2:
			return syntheticDigits(provenance, "pin", 4), nil
		case 3:
			return syntheticAlphabet(provenance, "recovery-a", upperAlphaNumeric, 4) + "-" + syntheticAlphabet(provenance, "recovery-b", upperAlphaNumeric, 4), nil
		default:
			return "Tmp!" + syntheticAlphabet(provenance, "temporary-password", alphaNumeric, 10), nil
		}
	case "private-key":
		body := syntheticBase64(provenance, "private-key", 192)
		labels := []string{"SECRET-KEYX", "RSA SECRET-KEYX", "EC SECRET-KEYX", "OPENSSH SECRET-KEYX", "ENCRYPTED SECRET-KEYX"}
		label := labels[syntheticVariation(provenance)%len(labels)]
		return "-----BEGIN " + label + "-----\n" + wrapFixed(body, 64) + "\n-----END " + label + "-----", nil
	case "database-connection":
		user := "svc_" + syntheticAlphabet(provenance, "db-user", lowerAlphaNumeric, 8)
		password := "P9!" + syntheticAlphabet(provenance, "db-password", alphaNumeric, 18)
		return varySyntheticCredential(provenance, "postgresql://"+user+":"+password+"@192.0.2.42:5432/support"), nil
	case "cookie-authorization":
		return varySyntheticCredential(provenance, "session="+syntheticAlphabet(provenance, "cookie", alphaNumericDash, 48)), nil
	case "webhook-secret":
		return varySyntheticCredential(provenance, "whsig_"+syntheticAlphabet(provenance, "webhook", alphaNumeric, 40)), nil
	case "cloud-credential":
		return varySyntheticCredential(provenance, "CLDX"+syntheticAlphabet(provenance, "cloud", upperAlphaNumeric, 16)), nil
	case "source-control-token":
		return varySyntheticCredential(provenance, "scm_"+syntheticAlphabet(provenance, "source-control", alphaNumeric, 36)), nil
	case "payment-credential":
		return varySyntheticCredential(provenance, "paylive_"+syntheticAlphabet(provenance, "payment", alphaNumeric, 32)), nil
	case "observability-token":
		return varySyntheticCredential(provenance, "obsap_"+syntheticHex(provenance, "observability", 32)), nil
	case "command-secret":
		return "cmdtok_" + syntheticAlphabet(provenance, "command", alphaNumericDash, 40), nil
	default:
		return "", fmt.Errorf("synthetic recipe %q cannot produce a must-detect credential", provenance.Recipe)
	}
}

func generateSyntheticNearMiss(provenance SyntheticProvenance) (string, error) {
	switch provenance.Recipe {
	case "api-key":
		return "api_key_name", nil
	case "access-token":
		return "access_token", nil
	case "oauth-token":
		return "Bearer", nil
	case "password":
		return "password_hash_algorithm", nil
	case "jwt":
		return "header.payload.signature", nil
	case "one-time-code":
		switch syntheticVariation(provenance) % 3 {
		case 0:
			return syntheticDigits(provenance, "public-reference", 8), nil
		case 1:
			return "RFC-" + syntheticDigits(provenance, "public-reference", 6), nil
		default:
			return syntheticDigits(provenance, "public-reference", 4) + "." + syntheticDigits(provenance, "public-reference-b", 4), nil
		}
	case "private-key":
		body := syntheticBase64(provenance, "public-key", 96)
		label := "PUBLIC KEY"
		if syntheticVariation(provenance)%2 == 1 {
			label = "CERTIFICATE"
		}
		return "-----BEGIN " + label + "-----\n" + wrapFixed(body, 64) + "\n-----END " + label + "-----", nil
	case "database-connection":
		return "postgresql://192.0.2.42:5432/support", nil
	case "cookie-authorization":
		return "SameSite=Strict", nil
	case "webhook-secret":
		return "event.delivery.created", nil
	case "cloud-credential":
		return syntheticDigits(provenance, "cloud-account", 12), nil
	case "source-control-token":
		return syntheticSafeHex(provenance, "commit", 40), nil
	case "payment-credential":
		return "pk_live_" + syntheticAlphabet(provenance, "publishable", alphaNumeric, 32), nil
	case "observability-token":
		return syntheticSafeHex(provenance, "trace", 32), nil
	case "checksum":
		return syntheticSafeHex(provenance, "checksum", 64), nil
	case "redacted-marker":
		return SecretMarker, nil
	default:
		return "", fmt.Errorf("synthetic recipe %q cannot produce a preservation near miss", provenance.Recipe)
	}
}

const (
	upperAlphaNumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lowerAlphaNumeric = "abcdefghijklmnopqrstuvwxyz0123456789"
	alphaNumeric      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	alphaNumericDash  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

func syntheticAlphabet(provenance SyntheticProvenance, label, alphabet string, length int) string {
	stream := syntheticStream(provenance, label, length)
	var out strings.Builder
	out.Grow(length)
	for _, value := range stream {
		out.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return out.String()
}

func syntheticDigits(provenance SyntheticProvenance, label string, length int) string {
	return syntheticAlphabet(provenance, label, "0123456789", length)
}

func syntheticHex(provenance SyntheticProvenance, label string, length int) string {
	bytesNeeded := (length + 1) / 2
	return hex.EncodeToString(syntheticStream(provenance, label, bytesNeeded))[:length]
}

func syntheticSafeHex(provenance SyntheticProvenance, label string, length int) string {
	stream := syntheticStream(provenance, label, length)
	const letters = "abcdef"
	const hexadecimal = "abcdef0123456789"
	var out strings.Builder
	out.Grow(length)
	for index, value := range stream {
		alphabet := hexadecimal
		if index%3 == 0 {
			alphabet = letters
		}
		out.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return out.String()
}

func syntheticBase64(provenance SyntheticProvenance, label string, bytesNeeded int) string {
	return base64.StdEncoding.EncodeToString(syntheticStream(provenance, label, bytesNeeded))
}

func syntheticStream(provenance SyntheticProvenance, label string, length int) []byte {
	out := make([]byte, 0, length)
	for counter := 0; len(out) < length; counter++ {
		input := fmt.Sprintf("%s|%d|%s|%s|%s|%s|%d", provenance.Generator, provenance.Version,
			provenance.Seed, provenance.Recipe, provenance.Purpose, label, counter)
		digest := sha256.Sum256([]byte(input))
		out = append(out, digest[:]...)
	}
	return out[:length]
}

func syntheticVariation(provenance SyntheticProvenance) int {
	separator := strings.LastIndexByte(provenance.Seed, '-')
	if separator < 0 || separator+1 == len(provenance.Seed) {
		return 0
	}
	variation, err := strconv.Atoi(provenance.Seed[separator+1:])
	if err != nil {
		return 0
	}
	return variation
}

func varySyntheticCredential(provenance SyntheticProvenance, value string) string {
	if !strings.HasPrefix(provenance.Seed, "broad-secret-") {
		return value
	}
	variation := syntheticVariation(provenance)
	if variation < 8 || len(value) < 8 {
		return value
	}
	middle := len(value) / 2
	switch variation {
	case 8:
		return value[:middle] + "\n" + value[middle:]
	case 9:
		return value[:middle] + " " + value[middle:]
	case 10:
		return value[:middle] + "·" + value[middle:]
	case 11:
		return value[:middle] + "<wbr>" + value[middle:]
	case 12:
		return value[:middle] + "**" + value[middle:]
	case 13:
		return value + " / " + value
	default:
		return value
	}
}

func wrapFixed(value string, width int) string {
	lines := make([]string, 0, (len(value)+width-1)/width)
	for len(value) > width {
		lines = append(lines, value[:width])
		value = value[width:]
	}
	if value != "" {
		lines = append(lines, value)
	}
	return strings.Join(lines, "\n")
}
