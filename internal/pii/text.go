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
	linkRe = regexp.MustCompile(`(?:(?:https?:\/\/)?(?:[a-z0-9.\-]+|www|[a-z0-9.\-])[.](?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))*\))+(?:\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))*\)|[^\s!()\[\]{};:'".,<>?]))`)
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
}

// RedactTextNotice is shown for freeform text when NER is not installed.
const RedactTextNotice = `[redacted — run "hs ner install" for content]`

// RedactText redacts free-form text using known identities followed by regex sweeps.
// When NER is available, names are detected via ML. Without NER, freeform text is
// hidden entirely (structured field redaction still works).
func (e *Engine) RedactText(text string, known []KnownIdentity) string {
	if !e.Enabled() || text == "" {
		return text
	}

	// Without NER: can't safely redact names in freeform text
	if e.ner == nil {
		return RedactTextNotice
	}

	// 1. Detect names via NER on ORIGINAL text (natural language, best accuracy)
	nerNames, _ := e.ner.DetectNames(text)

	// 2. Known identity replacement
	out, inserted := e.redactKnown(text, known)

	// 3. Replace NER-detected names not already handled by known identities
	for _, span := range nerNames {
		name := span.Text
		if inserted[name] {
			continue
		}
		skip := false
		for _, w := range strings.Fields(name) {
			if inserted[w] {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		fp := e.fakePersonForKey("name:" + canonical(name))
		fakeFull := fp.First + " " + fp.Last
		out = replaceWordInsensitive(out, name, fakeFull)
		inserted[fakeFull] = true
	}

	// 4. Regex sweep — email, phone, SSN, address, etc. (no name regex)
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
			default:
				return e.token(rule.kind, match)
			}
		})
	}
	return out
}

// redactKnown replaces known identity data with fake names (for name parts)
// and deterministic tokens (for emails, phones). The returned set tracks
// fake names inserted so the regex sweep can skip them.
func (e *Engine) redactKnown(text string, known []KnownIdentity) (string, map[string]bool) {
	out := text
	inserted := map[string]bool{}

	for _, id := range known {
		key := personKey(id.First, id.Last, id.Email)
		if key == "" && strings.TrimSpace(id.Phone) != "" {
			key = "phone:" + onlyDigits(id.Phone)
		}
		if key == "" {
			continue
		}

		fp := e.fakePersonForKey(key)
		fakeFull := fp.First + " " + fp.Last
		personToken := e.token("person", key)

		// 1. Full email & phone → tokens (safe from regex interference)
		if id.Email != "" {
			out = replaceLiteralInsensitive(out, id.Email, personToken)
		}
		if id.Phone != "" {
			out = replaceLiteralInsensitive(out, id.Phone, personToken)
		}

		// 2. Names → fake names
		fullName := strings.TrimSpace(strings.TrimSpace(id.First) + " " + strings.TrimSpace(id.Last))
		if fullName != "" {
			out = replaceWordInsensitive(out, fullName, fakeFull)
			inserted[fakeFull] = true
		}
		if len(strings.TrimSpace(id.First)) >= 3 {
			out = replaceWordInsensitive(out, id.First, fp.First)
			inserted[fp.First] = true
		}
		if len(strings.TrimSpace(id.Last)) >= 3 {
			out = replaceWordInsensitive(out, id.Last, fp.Last)
			inserted[fp.Last] = true
		}

		// 3. Email prefix as last resort — catches standalone uses like "hey alice"
		// that weren't already handled by name replacement above.
		if id.Email != "" {
			parts := strings.Split(id.Email, "@")
			if len(parts) > 0 && parts[0] != "" {
				out = replaceWordInsensitive(out, parts[0], fp.First)
			}
		}
	}
	return out, inserted
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
