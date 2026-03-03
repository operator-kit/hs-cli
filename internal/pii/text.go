package pii

import (
	"regexp"
	"strings"
)

type textRule struct {
	kind string
	re   *regexp.Regexp
}

var (
	emailRe = regexp.MustCompile(`(?i)([A-Za-z0-9!#$%&'*+\/=?^_{|.}~-]+@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)`)

	phoneWithExtRe = regexp.MustCompile(`(?i)(?:(?:\+?1\s*(?:[.-]\s*)?)?(?:\(\s*(?:[2-9]1[02-9]|[2-9][02-8]1|[2-9][02-8][02-9])\s*\)|(?:[2-9]1[02-9]|[2-9][02-8]1|[2-9][02-8][02-9]))\s*(?:[.-]\s*)?)?(?:[2-9]1[02-9]|[2-9][02-9]1|[2-9][02-9]{2})\s*(?:[.-]\s*)?(?:[0-9]{4})(?:\s*(?:#|x\.?|ext\.?|extension)\s*(?:\d+)?)`)
	phoneRe        = regexp.MustCompile(`(?:(?:\+?\d{1,3}[-.\s*]?)?(?:\(?\d{3}\)?[-.\s*]?)?\d{3}[-.\s*]?\d{4,6})|(?:(?:(?:\(\+?\d{2}\))|(?:\+?\d{2}))\s*\d{2}\s*\d{3}\s*\d{4})`)

	ssnRe        = regexp.MustCompile(`\d{3}[- ]?\d{2}[- ]?\d{4}`)
	ssnContextRe = regexp.MustCompile(`(?i)SSN|social security`)

	creditCardRe = regexp.MustCompile(`(?:(?:(?:\d{4}[- ]?){3}\d{4}|\d{15,16}))`)
	ibanRe       = regexp.MustCompile(`[A-Z]{2}\d{2}[A-Z0-9]{4}\d{7}([A-Z\d]?){0,16}`)
	btcRe        = regexp.MustCompile(`[13][a-km-zA-HJ-NP-Z1-9]{25,34}`)
	bech32Re     = regexp.MustCompile(`(?i)bc1[ac-hj-np-z02-9]{6,87}`)

	ipv4Re = regexp.MustCompile(`(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)
	ipv6Re = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{1,4}:){2,7}[0-9A-Fa-f]{1,4}\b`)
	macRe  = regexp.MustCompile(`(([a-fA-F0-9]{2}[:-]){5}([a-fA-F0-9]{2}))`)

	addressContextRe = regexp.MustCompile(`(?i)(lives at|located at|resides at|found at|situated at|at address|address is|at location|based at) (\d+[^\n\.]*?(Street|St|Avenue|Ave|Road|Rd|Drive|Dr|Lane|Ln|Place|Pl|Boulevard|Blvd|Way)[^\n\.]*)`)
	addressLabelRe   = regexp.MustCompile(`(?i)(:\s+|at\s+|@\s+)(\d+[-\s]?\w*|\d+-\d+-\d+)[\s,]+([A-Za-z\p{L}]+([\s'-][A-Za-z\p{L}]+)*[\s,]+)+(Road|Rd|Street|St|Avenue|Ave|Boulevard|Blvd|Drive|Dr|Lane|Ln|Place|Pl|Rue|Via|Viale|Strasse|Straße|Calle|Avenida)`)
	addressMainRe    = regexp.MustCompile(`(?i)(\d+[-\s]?\w*|\d+-\d+-\d+)[\s,]+([A-Za-z\p{L}]+([\s'-][A-Za-z\p{L}]+)*[\s,]+)+(Street|St|Avenue|Ave|Road|Rd|Drive|Dr|Lane|Ln|Place|Pl|Boulevard|Blvd|Way|Plaza|Square|Sq|Court|Ct|Terrace|Ter|Circle|Cir|Alley|Row|Highway|Hwy|Parkway|Pkwy|Path|Trail|Tr|Crescent|Cres|Rue|Strasse|Straße|Calle|Via|Viale|Avenida|Carrer|Straat|Gasse|Weg|Camino|Ulica|Utca|Prospekt|Dori|Jalan|Marg|Dao|Jie|Lu|út|de la|del|di|van|von)\b`)
	zipRe            = regexp.MustCompile(`\b\d{5}(?:[-\s]\d{4})?\b`)
	poBoxRe          = regexp.MustCompile(`(?i)P\.? ?O\.? Box \d+`)
	linkRe           = regexp.MustCompile(`(?:(?:https?:\/\/)?(?:[a-z0-9.\-]+|www|[a-z0-9.\-])[.](?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))*\))+(?:\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))*\)|[^\s!()\[\]{};:'".,<>?]))`)
	nameRe           = regexp.MustCompile(`\b[A-Z][a-z]+ [A-Z][a-z]+\b`)
)

var textRules = []textRule{
	{kind: "email", re: emailRe},
	{kind: "phone", re: phoneWithExtRe},
	{kind: "phone", re: phoneRe},
	{kind: "ssn", re: ssnRe},
	{kind: "credit_card", re: creditCardRe},
	{kind: "iban", re: ibanRe},
	{kind: "btc", re: btcRe},
	{kind: "btc", re: bech32Re},
	{kind: "ipv4", re: ipv4Re},
	{kind: "ipv6", re: ipv6Re},
	{kind: "mac", re: macRe},
	{kind: "address", re: addressContextRe},
	{kind: "address", re: addressLabelRe},
	{kind: "address", re: addressMainRe},
	{kind: "zip", re: zipRe},
	{kind: "po_box", re: poBoxRe},
	{kind: "url", re: linkRe},
	{kind: "name", re: nameRe},
}

// RedactText redacts free-form text using known identities followed by regex sweeps.
func (e *Engine) RedactText(text string, known []KnownIdentity) string {
	if !e.Enabled() || text == "" {
		return text
	}

	out := e.redactKnown(text, known)
	ssnContext := ssnContextRe.MatchString(out)

	for _, rule := range textRules {
		out = applyRegexWithContext(out, rule.re, func(match string, start, end int, full string) string {
			switch rule.kind {
			case "ssn":
				rawDigits := onlyDigits(match)
				formatted := strings.ContainsAny(match, "- ")
				if !(formatted || (ssnContext && len(rawDigits) == 9)) {
					return match
				}
				return e.token(rule.kind, rawDigits)
			case "name":
				if looksLikeAddressContext(full, start, end) {
					return match
				}
				return e.token(rule.kind, match)
			default:
				return e.token(rule.kind, match)
			}
		})
	}
	return out
}

func (e *Engine) redactKnown(text string, known []KnownIdentity) string {
	out := text
	for _, id := range known {
		key := personKey(id.First, id.Last, id.Email)
		if key == "" && strings.TrimSpace(id.Phone) != "" {
			key = "phone:" + onlyDigits(id.Phone)
		}
		if key == "" {
			continue
		}

		personToken := e.token("person", key)

		if id.Email != "" {
			out = replaceLiteralInsensitive(out, id.Email, personToken)
			parts := strings.Split(id.Email, "@")
			if len(parts) > 0 && parts[0] != "" {
				out = replaceWordInsensitive(out, parts[0], personToken)
			}
		}
		if id.Phone != "" {
			out = replaceLiteralInsensitive(out, id.Phone, personToken)
		}

		fullName := strings.TrimSpace(strings.TrimSpace(id.First) + " " + strings.TrimSpace(id.Last))
		if fullName != "" {
			out = replaceWordInsensitive(out, fullName, personToken)
		}
		if len(strings.TrimSpace(id.First)) >= 3 {
			out = replaceWordInsensitive(out, id.First, personToken)
		}
		if len(strings.TrimSpace(id.Last)) >= 3 {
			out = replaceWordInsensitive(out, id.Last, personToken)
		}
	}
	return out
}

func looksLikeAddressContext(full string, start, end int) bool {
	const span = 36
	lo := start - span
	if lo < 0 {
		lo = 0
	}
	hi := end + span
	if hi > len(full) {
		hi = len(full)
	}
	window := strings.ToLower(full[lo:hi])
	addressWords := []string{
		"street", "st ", "avenue", "road", "drive", "lane", "boulevard", "blvd", "suite", "apt", "po box",
		"address", "city", "zip", "postal", "located at", "lives at",
	}
	for _, w := range addressWords {
		if strings.Contains(window, w) {
			return true
		}
	}
	return false
}

func applyRegexWithContext(text string, re *regexp.Regexp, fn func(match string, start, end int, full string) string) string {
	idxs := re.FindAllStringIndex(text, -1)
	if len(idxs) == 0 {
		return text
	}

	var b strings.Builder
	last := 0
	for _, idx := range idxs {
		start, end := idx[0], idx[1]
		if start < last {
			continue
		}
		b.WriteString(text[last:start])
		match := text[start:end]
		b.WriteString(fn(match, start, end, text))
		last = end
	}
	b.WriteString(text[last:])
	return b.String()
}

func replaceLiteralInsensitive(text, literal, replacement string) string {
	literal = strings.TrimSpace(literal)
	if literal == "" {
		return text
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(literal))
	return re.ReplaceAllString(text, replacement)
}

func replaceWordInsensitive(text, literal, replacement string) string {
	literal = strings.TrimSpace(literal)
	if literal == "" {
		return text
	}
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(literal) + `\b`)
	return re.ReplaceAllString(text, replacement)
}

