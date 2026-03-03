package pii

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

type KnownIdentity struct {
	Type  string
	First string
	Last  string
	Email string
	Phone string
}

type Engine struct {
	mode   string
	secret string

	mu      sync.RWMutex
	people  map[string]fakePerson
	replace map[string]string
}

type fakePerson struct {
	First string
	Last  string
	Email string
}

func NewEngine(mode, secret string) *Engine {
	return &Engine{
		mode:    NormalizeMode(mode),
		secret:  secret,
		people:  map[string]fakePerson{},
		replace: map[string]string{},
	}
}

func (e *Engine) Mode() string {
	return e.mode
}

func (e *Engine) Enabled() bool {
	return IsEnabled(e.mode)
}

func (e *Engine) ShouldRedactType(entityType string) bool {
	return ShouldRedactType(e.mode, entityType)
}

func (e *Engine) RedactPerson(first, last, email string) (string, string, string) {
	key := personKey(first, last, email)
	if key == "" {
		return first, last, email
	}

	fp := e.fakePersonForKey(key)
	outFirst := first
	outLast := last
	outEmail := email

	if strings.TrimSpace(first) != "" || strings.TrimSpace(last) != "" {
		outFirst = fp.First
		outLast = fp.Last
	}
	if strings.TrimSpace(email) != "" {
		outEmail = fp.Email
	}
	return outFirst, outLast, outEmail
}

func (e *Engine) RedactEmail(email string) string {
	if strings.TrimSpace(email) == "" {
		return email
	}
	_, _, fakeEmail := e.RedactPerson("", "", email)
	return fakeEmail
}

func (e *Engine) RedactPhone(phone string) string {
	digits := onlyDigits(phone)
	if digits == "" {
		return phone
	}
	sum := e.hashBytes("phone|" + digits)
	redactedDigits := make([]byte, len(digits))
	for i := range digits {
		d := (sum[i%len(sum)] + byte(i)) % 10
		if i == 0 && d == 0 {
			d = 7
		}
		redactedDigits[i] = '0' + d
	}

	out := make([]rune, 0, len(phone))
	idx := 0
	for _, r := range phone {
		if unicode.IsDigit(r) {
			out = append(out, rune(redactedDigits[idx]))
			idx++
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func (e *Engine) token(kind, raw string) string {
	key := strings.ToLower(strings.TrimSpace(kind)) + "|" + canonical(raw)

	e.mu.RLock()
	if v, ok := e.replace[key]; ok {
		e.mu.RUnlock()
		return v
	}
	e.mu.RUnlock()

	sum := e.hashBytes(key)
	token := fmt.Sprintf("[[%s_%s]]", kind, hex.EncodeToString(sum[:2]))

	e.mu.Lock()
	e.replace[key] = token
	e.mu.Unlock()

	return token
}

func (e *Engine) hashBytes(v string) [32]byte {
	if strings.TrimSpace(e.secret) == "" {
		return sha256.Sum256([]byte(v))
	}
	h := hmac.New(sha256.New, []byte(e.secret))
	h.Write([]byte(v))
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (e *Engine) fakePersonForKey(key string) fakePerson {
	e.mu.RLock()
	if v, ok := e.people[key]; ok {
		e.mu.RUnlock()
		return v
	}
	e.mu.RUnlock()

	sum := e.hashBytes("person|" + key)
	first := firstNames[int(sum[0])%len(firstNames)]
	last := lastNames[int(sum[1])%len(lastNames)]
	suffix := hex.EncodeToString(sum[2:4])
	email := fmt.Sprintf("%s.%s-%s@anon.local", slugPart(first), slugPart(last), suffix)

	fp := fakePerson{
		First: first,
		Last:  last,
		Email: email,
	}

	e.mu.Lock()
	e.people[key] = fp
	e.mu.Unlock()
	return fp
}

func personKey(first, last, email string) string {
	if c := canonicalEmail(email); c != "" {
		return "email:" + c
	}
	full := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
	if c := canonical(full); c != "" {
		return "name:" + c
	}
	return ""
}

func canonical(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func canonicalEmail(v string) string {
	return canonical(v)
}

var nonDigitRe = regexp.MustCompile(`\D+`)

func onlyDigits(v string) string {
	return nonDigitRe.ReplaceAllString(v, "")
}

var slugPartRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugPart(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = slugPartRe.ReplaceAllString(s, ".")
	s = strings.Trim(s, ".")
	if s == "" {
		return "anon"
	}
	return s
}

var firstNames = []string{
	"Alex", "Avery", "Blake", "Casey", "Charlie", "Dakota", "Drew", "Elliot", "Emerson", "Finley",
	"Harper", "Hayden", "Indigo", "Jamie", "Jordan", "Kai", "Kendall", "Lane", "Logan", "Morgan",
	"Noel", "Parker", "Quinn", "Reese", "Remy", "River", "Rowan", "Rylan", "Sage", "Sawyer",
	"Shawn", "Skyler", "Spencer", "Taylor", "Terry", "Winter", "Wren", "Ari", "Micah", "Nico",
	"Ash", "Cameron", "Jules", "Kris", "Lee", "Milan", "Robin", "Shay", "Tatum", "Zion",
}

var lastNames = []string{
	"Adams", "Baker", "Barnes", "Bennett", "Brooks", "Campbell", "Carter", "Collins", "Cooper", "Cruz",
	"Davis", "Diaz", "Edwards", "Evans", "Foster", "Garcia", "Gomez", "Gray", "Green", "Hall",
	"Harris", "Hayes", "Hill", "Howard", "Hughes", "James", "Jenkins", "Kelly", "King", "Lee",
	"Lewis", "Long", "Lopez", "Martin", "Miller", "Mitchell", "Moore", "Morgan", "Morris", "Nelson",
	"Parker", "Perry", "Price", "Reed", "Rivera", "Roberts", "Russell", "Stewart", "Taylor", "Ward",
}

