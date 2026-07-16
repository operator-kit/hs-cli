package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/operator-kit/hs-cli/internal/config"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/pii"
	"github.com/operator-kit/hs-cli/internal/pii/ner"
	"github.com/operator-kit/hs-cli/internal/pii/secretstore"
	piisetup "github.com/operator-kit/hs-cli/internal/pii/setup"
	"github.com/operator-kit/hs-cli/internal/types"
)

var (
	customerPIIContext     = pii.JSONContext{RootEntity: "customer", Resource: pii.ResourceCustomer}
	userPIIContext         = pii.JSONContext{RootEntity: "user", Resource: pii.ResourceUser}
	conversationPIIContext = pii.JSONContext{Resource: pii.ResourceConversation}
	ratingPIIContext       = pii.JSONContext{Resource: pii.ResourceRating}
	reportPIIContext       = pii.JSONContext{Resource: pii.ResourceReport}
	attachmentPIIContext   = pii.JSONContext{Resource: pii.ResourceAttachment}

	resolvePIIContext     = defaultResolvePIIContext
	invocationPIIPrepared bool
	invocationPIIMode     pii.Mode
	invocationPIIContext  pii.PseudonymContext
)

func effectivePIIMode() (pii.Mode, error) {
	mode := ""
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
	pseudonym, err := contextForPIIMode(context.Background(), mode)
	if err != nil {
		return nil, err
	}
	var opts []pii.EngineOption
	if ner.IsModelReady() {
		d, nerErr := ner.NewDetector()
		if nerErr == nil {
			opts = append(opts, pii.WithNER(d))
		}
	}
	engine, err := pii.NewEngine(mode, pseudonym, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating PII engine: %w", err)
	}
	return engine, nil
}

func defaultResolvePIIContext(ctx context.Context, mode pii.Mode, configPath string) (pii.PseudonymContext, error) {
	path := config.ResolvedPath(configPath)
	resolver := &piisetup.SecretResolver{
		Store: secretstore.NewKeyringStore(),
		Lock:  secretstore.NewFileLock(filepath.Join(filepath.Dir(path), ".pii-secret.lock")),
	}
	return resolver.ResolveContext(ctx, mode)
}

func resetPIIInvocation() {
	invocationPIIPrepared = false
	invocationPIIMode = pii.ModeOff
	invocationPIIContext = pii.PseudonymContext{}
}

func preflightPIISecret(ctx context.Context) error {
	mode, err := effectivePIIMode()
	if err != nil {
		return err
	}
	_, err = contextForPIIMode(ctx, mode)
	return err
}

func contextForPIIMode(ctx context.Context, mode pii.Mode) (pii.PseudonymContext, error) {
	if invocationPIIPrepared && invocationPIIMode == mode {
		return invocationPIIContext, nil
	}
	if !pii.IsEnabled(mode) {
		invocationPIIPrepared = true
		invocationPIIMode = mode
		invocationPIIContext = pii.PseudonymContext{}
		return pii.PseudonymContext{}, nil
	}

	pseudonym, err := resolvePIIContext(ctx, mode, cfgPath)
	if err != nil {
		return pii.PseudonymContext{}, fmt.Errorf("resolving PII redaction key: %w", err)
	}
	invocationPIIPrepared = true
	invocationPIIMode = mode
	invocationPIIContext = pseudonym
	return pseudonym, nil
}

// redactRawWithPII is the mandatory presentation boundary for Inbox JSON. Once
// redaction is enabled it fails closed: malformed or uninspectable data is
// never passed through unchanged.
func redactRawWithPII(data json.RawMessage, contexts ...pii.JSONContext) (json.RawMessage, error) {
	engine, err := newPIIEngine()
	if err != nil {
		return nil, err
	}
	if !engine.Enabled() {
		return data, nil
	}

	ctx := pii.JSONContext{}
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	redacted, err := engine.RedactJSONWithContext(data, ctx)
	if err != nil {
		return nil, fmt.Errorf("redacting Inbox output: %w", err)
	}
	return redacted, nil
}

func printRawWithPII(data json.RawMessage, contexts ...pii.JSONContext) error {
	redacted, err := redactRawWithPII(data, contexts...)
	if err != nil {
		return err
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
