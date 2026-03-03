package cmd

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/pii"
	"github.com/operator-kit/hs-cli/internal/types"
)

func effectivePIIMode() (string, error) {
	mode := pii.ModeOff
	allowUnredacted := false
	if cfg != nil {
		mode = cfg.InboxPIIMode
		allowUnredacted = cfg.InboxPIIAllowUnredacted
	}
	return pii.EffectiveMode(mode, allowUnredacted, unredacted)
}

func newPIIEngine() (*pii.Engine, error) {
	mode, err := effectivePIIMode()
	if err != nil {
		return nil, err
	}
	return pii.NewEngine(mode, os.Getenv("HS_INBOX_PII_SECRET")), nil
}

func printRawWithPII(data json.RawMessage) error {
	engine, err := newPIIEngine()
	if err != nil {
		return err
	}
	if !engine.Enabled() {
		return output.PrintRaw(data)
	}
	redacted, err := engine.RedactJSON(data)
	if err != nil {
		// Preserve existing behavior for non-JSON payloads.
		return output.PrintRaw(data)
	}
	return output.PrintRaw(redacted)
}

func redactTextWithPII(engine *pii.Engine, text string, known ...pii.KnownIdentity) string {
	if engine == nil || !engine.Enabled() || text == "" {
		return text
	}
	return engine.RedactText(text, known)
}

func redactPersonForOutput(engine *pii.Engine, person *types.Person, defaultType string) {
	if engine == nil || !engine.Enabled() || person == nil {
		return
	}

	entityType := strings.TrimSpace(person.Type)
	if entityType == "" {
		entityType = defaultType
	}
	if !engine.ShouldRedactType(entityType) {
		return
	}
	if person.ID == 0 && strings.EqualFold(person.Email, "unknown") {
		return
	}

	person.First, person.Last, person.Email = engine.RedactPerson(person.First, person.Last, person.Email)
}

func redactCustomerForOutput(engine *pii.Engine, customer *types.Customer) {
	if engine == nil || !engine.Enabled() || customer == nil {
		return
	}
	if !engine.ShouldRedactType("customer") {
		return
	}
	customer.FirstName, customer.LastName, customer.Email = engine.RedactPerson(customer.FirstName, customer.LastName, customer.Email)
	if customer.Phone != "" {
		customer.Phone = engine.RedactPhone(customer.Phone)
	}
	for i := range customer.Emails {
		customer.Emails[i].Value = engine.RedactEmail(customer.Emails[i].Value)
	}
}

func redactUserForOutput(engine *pii.Engine, user *types.User) {
	if engine == nil || !engine.Enabled() || user == nil {
		return
	}
	if !engine.ShouldRedactType("user") {
		return
	}
	user.FirstName, user.LastName, user.Email = engine.RedactPerson(user.FirstName, user.LastName, user.Email)
}

func knownFromPerson(person types.Person, defaultType string) pii.KnownIdentity {
	entityType := strings.TrimSpace(person.Type)
	if entityType == "" {
		entityType = defaultType
	}
	return pii.KnownIdentity{
		Type:  entityType,
		First: person.First,
		Last:  person.Last,
		Email: person.Email,
	}
}

func knownFromCustomer(customer types.Customer) pii.KnownIdentity {
	return pii.KnownIdentity{
		Type:  "customer",
		First: customer.FirstName,
		Last:  customer.LastName,
		Email: customer.Email,
		Phone: customer.Phone,
	}
}

func threadAuthorType(threadType string) string {
	switch strings.ToLower(strings.TrimSpace(threadType)) {
	case "customer", "chat", "beaconchat", "phone":
		return "customer"
	default:
		return "user"
	}
}
