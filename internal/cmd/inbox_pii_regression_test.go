package cmd

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const criticalPIIEmail = "alice.critical@example.com"

func executePIIFixtureCommand(
	t *testing.T,
	mock *mockClient,
	piiMode string,
	outputFormat string,
	execute func() error,
) string {
	t.Helper()

	previousOutput := output.Out
	saveRestore(t)
	buf := setupTest(mock)
	t.Cleanup(func() { output.Out = previousOutput })

	cfg.InboxPIIMode = piiMode
	format = outputFormat
	require.NoError(t, execute())
	return buf.String()
}

func TestPIIRegression_Critical01_CustomersModeRedactsTopLevelCustomerJSON(t *testing.T) {
	const customer = `{
		"id": 101,
		"firstName": "Alice",
		"lastName": "Critical",
		"email": "alice.critical@example.com",
		"phone": "+1 415 555 0199"
	}`

	surfaces := []struct {
		name    string
		newMock func() *mockClient
		args    []string
	}{
		{
			name: "list",
			newMock: func() *mockClient {
				return &mockClient{
					ListCustomersFn: func(context.Context, url.Values) (json.RawMessage, error) {
						return halJSON("customers", "["+customer+"]"), nil
					},
				}
			},
			args: []string{"list"},
		},
		{
			name: "get",
			newMock: func() *mockClient {
				return &mockClient{
					GetCustomerFn: func(context.Context, string, url.Values) (json.RawMessage, error) {
						return json.RawMessage(customer), nil
					},
				}
			},
			args: []string{"get", "101"},
		},
	}

	for _, outputFormat := range []string{"json", "json-full"} {
		for _, surface := range surfaces {
			outputFormat := outputFormat
			surface := surface
			t.Run(surface.name+"/"+outputFormat, func(t *testing.T) {
				command := newCustomersCmd()
				command.SetArgs(surface.args)
				out := executePIIFixtureCommand(t, surface.newMock(), "customers", outputFormat, command.Execute)

				for _, rawPII := range []string{"Alice", "Critical", criticalPIIEmail, "+1 415 555 0199"} {
					assert.NotContains(t, out, rawPII, "top-level customer PII must be redacted in customers mode")
				}
			})
		}
	}
}

func TestPIIRegression_Critical02_PIIBearingCommandsCannotBypassRedaction(t *testing.T) {
	t.Run("rating comments", func(t *testing.T) {
		mock := &mockClient{
			GetRatingFn: func(context.Context, string) (json.RawMessage, error) {
				return json.RawMessage(`{
					"id": 7,
					"rating": "great",
					"comments": "Please follow up with alice.critical@example.com"
				}`), nil
			},
		}
		command := newRatingsCmd()
		command.SetArgs([]string{"get", "7"})
		out := executePIIFixtureCommand(t, mock, "all", "json-full", command.Execute)

		assert.NotContains(t, out, criticalPIIEmail, "rating comments must pass through the PII output boundary")
	})

	t.Run("report payloads", func(t *testing.T) {
		mock := &mockClient{
			GetReportFn: func(context.Context, string, url.Values) (json.RawMessage, error) {
				return json.RawMessage(`{
					"customers": [{"id": 101, "email": "alice.critical@example.com"}]
				}`), nil
			},
		}
		command := newReportsCmd()
		command.SetArgs([]string{"customers"})
		out := executePIIFixtureCommand(t, mock, "all", "json-full", command.Execute)

		assert.NotContains(t, out, criticalPIIEmail, "report payloads must pass through the PII output boundary")
	})

	t.Run("attachment metadata", func(t *testing.T) {
		mock := &mockClient{
			GetAttachmentDataFn: func(context.Context, string, string) (json.RawMessage, error) {
				return json.RawMessage(`{
					"filename": "alice.critical@example.com",
					"mimeType": "text/plain",
					"data": "c2Vuc2l0aXZl"
				}`), nil
			},
		}
		command := newConversationAttachmentsCmd()
		command.SetArgs([]string{"get", "42", "9"})
		out := executePIIFixtureCommand(t, mock, "all", "json-full", command.Execute)

		assert.NotContains(t, out, criticalPIIEmail, "attachment metadata must pass through the PII output boundary")
	})

	t.Run("conversation custom-field values", func(t *testing.T) {
		mock := &mockClient{
			GetConversationFn: func(context.Context, string, url.Values) (json.RawMessage, error) {
				return json.RawMessage(`{
					"id": 42,
					"number": 1042,
					"subject": "Escalation",
					"customFields": [{
						"id": 17,
						"name": "Escalation contact",
						"value": "alice.critical@example.com"
					}]
				}`), nil
			},
		}
		command := conversationsGetCmd()
		command.SetArgs([]string{"42"})
		out := executePIIFixtureCommand(t, mock, "all", "json-full", command.Execute)

		assert.NotContains(t, out, criticalPIIEmail, "custom-field values must be scanned for PII")
	})
}
