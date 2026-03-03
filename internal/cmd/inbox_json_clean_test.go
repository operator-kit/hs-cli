package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanConversation(t *testing.T) {
	raw := json.RawMessage(`{
		"_links": {"self": {"href": "https://api.helpscout.net/v2/conversations/1"}},
		"_embedded": {"threads": []},
		"id": 1,
		"number": 100,
		"subject": "Help me",
		"status": "active",
		"state": "published",
		"type": "email",
		"mailboxId": 239969,
		"closedBy": 0,
		"closedByUser": {"id": 0, "email": "unknown", "first": "unknown", "last": "unknown"},
		"createdBy": {"id": 1, "email": "alice@test.com", "first": "Alice", "last": "Smith", "type": "customer"},
		"primaryCustomer": {"id": 1, "email": "alice@test.com", "first": "Alice", "last": "Smith", "type": "customer", "photoUrl": "http://photo.jpg"},
		"customerWaitingSince": {"friendly": "5 min ago", "time": "2025-01-01T10:00:00Z"},
		"userUpdatedAt": "2025-01-01T11:00:00Z",
		"source": {"type": "api", "via": "customer"},
		"threads": 2,
		"tags": [],
		"customFields": [],
		"cc": [],
		"bcc": []
	}`)

	result := cleanRawObject(raw, cleanConversation)

	// Dropped
	assert.Nil(t, result["_links"])
	assert.Nil(t, result["_embedded"])
	assert.Nil(t, result["state"])           // "published" dropped
	assert.Nil(t, result["closedBy"])        // sentinel 0
	assert.Nil(t, result["closedByUser"])    // sentinel
	assert.Nil(t, result["createdBy"])       // duplicate
	assert.Nil(t, result["primaryCustomer"]) // replaced by "customer"
	assert.Nil(t, result["userUpdatedAt"])   // renamed
	assert.Nil(t, result["tags"])            // empty
	assert.Nil(t, result["customFields"])    // empty
	assert.Nil(t, result["cc"])              // empty
	assert.Nil(t, result["bcc"])             // empty

	// Transformed
	assert.Equal(t, "Alice Smith (alice@test.com)", result["customer"])
	assert.Equal(t, "2025-01-01T11:00:00Z", result["updatedAt"])
	assert.Equal(t, float64(2), result["threadCount"])
	assert.Equal(t, "2025-01-01T10:00:00Z", result["customerWaitingSince"])

	// Kept
	assert.Equal(t, float64(1), result["id"])
	assert.Equal(t, float64(100), result["number"])
	assert.Equal(t, "Help me", result["subject"])
	assert.Equal(t, "active", result["status"])
}

func TestCleanConversationWithEmbeddedThreads(t *testing.T) {
	raw := json.RawMessage(`{
		"_links": {},
		"_embedded": {"threads": [
			{"id": 10, "type": "customer", "state": "published",
			 "body": "<p>Hello</p>",
			 "createdBy": {"email": "alice@test.com", "first": "Alice", "last": "S"},
			 "customer": {"email": "alice@test.com"},
			 "_links": {}, "_embedded": {"attachments": []},
			 "action": {"type": "default", "associatedEntities": {}},
			 "to": [], "cc": [], "bcc": []}
		]},
		"id": 1, "number": 100, "subject": "Test", "status": "active",
		"state": "published", "type": "email",
		"primaryCustomer": {"email": "alice@test.com", "first": "Alice", "last": "S"},
		"threads": 1, "userUpdatedAt": "2025-01-01T10:00:00Z",
		"closedBy": 0, "closedByUser": {"id": 0, "email": "unknown"}
	}`)

	result := cleanRawObject(raw, cleanConversation)

	// Threads hoisted and cleaned
	threads, ok := result["threads"].([]map[string]any)
	require.True(t, ok, "threads should be array of cleaned maps")
	require.Len(t, threads, 1)

	thread := threads[0]
	assert.Equal(t, "customer", thread["type"])
	assert.Equal(t, "Hello", thread["body"])                    // HTML→md
	assert.Equal(t, "Alice S (alice@test.com)", thread["from"]) // flattened
	assert.Nil(t, thread["_links"])
	assert.Nil(t, thread["customer"]) // duplicate dropped
	assert.Nil(t, thread["state"])    // "published" dropped
	assert.Nil(t, thread["action"])   // default dropped
}

func TestCleanThread(t *testing.T) {
	t.Run("customer thread", func(t *testing.T) {
		m := map[string]any{
			"_links":    map[string]any{"self": "..."},
			"_embedded": map[string]any{"attachments": []any{}},
			"id":        float64(10),
			"type":      "customer",
			"state":     "published",
			"body":      "<p>Hello <strong>world</strong></p>",
			"createdBy": map[string]any{"email": "alice@test.com", "first": "Alice", "last": "S"},
			"customer":  map[string]any{"email": "alice@test.com"},
			"action":    map[string]any{"type": "default", "associatedEntities": map[string]any{}},
			"to":        []any{},
			"cc":        []any{},
			"bcc":       []any{},
		}
		result := cleanThread(m)

		assert.Nil(t, result["_links"])
		assert.Nil(t, result["_embedded"])
		assert.Nil(t, result["customer"])
		assert.Nil(t, result["state"])
		assert.Nil(t, result["action"])
		assert.Nil(t, result["to"])
		assert.Equal(t, "Alice S (alice@test.com)", result["from"])
		assert.Contains(t, result["body"], "**world**") // HTML→md
	})

	t.Run("lineitem", func(t *testing.T) {
		m := map[string]any{
			"_links":    map[string]any{},
			"_embedded": map[string]any{"attachments": []any{}},
			"id":        float64(20),
			"type":      "lineitem",
			"status":    "active",
			"createdBy": map[string]any{"email": "ross@test.com", "first": "Ross", "last": "M"},
			"action":    map[string]any{"type": "default", "text": "You marked as Active", "associatedEntities": map[string]any{}},
			"source":    map[string]any{"type": "web", "via": "user"},
			"to":        []any{},
		}
		result := cleanThread(m)

		assert.Equal(t, "You marked as Active", result["action"])
		assert.Equal(t, "Ross M (ross@test.com)", result["from"])
		assert.Nil(t, result["source"]) // dropped for lineitems
		assert.Nil(t, result["body"])   // lineitems have no body
	})

	t.Run("draft note", func(t *testing.T) {
		m := map[string]any{
			"_links":     map[string]any{},
			"_embedded":  map[string]any{"attachments": []any{}},
			"id":         float64(30),
			"type":       "note",
			"state":      "draft",
			"status":     "active",
			"body":       "draft note<br>",
			"createdBy":  map[string]any{"email": "ross@test.com", "first": "Ross", "last": "M"},
			"customer":   map[string]any{"email": "customer@test.com"},
			"assignedTo": map[string]any{"email": "ross@test.com", "first": "Ross", "last": "M"},
			"action":     map[string]any{"type": "original-author", "associatedEntities": map[string]any{"user": float64(1)}},
			"to":         []any{},
		}
		result := cleanThread(m)

		assert.Equal(t, "draft", result["state"]) // preserved
		assert.Equal(t, "Ross M (ross@test.com)", result["from"])
		assert.Equal(t, "Ross M (ross@test.com)", result["assignedTo"])
		assert.Nil(t, result["customer"]) // duplicate dropped
	})

	t.Run("thread with attachments", func(t *testing.T) {
		m := map[string]any{
			"_links": map[string]any{},
			"_embedded": map[string]any{"attachments": []any{
				map[string]any{
					"_links":   map[string]any{"data": "..."},
					"id":       float64(55),
					"filename": "invoice.pdf",
					"mimeType": "application/pdf",
					"size":     float64(43008),
					"state":    "valid",
					"width":    float64(0),
					"height":   float64(0),
				},
			}},
			"id":        float64(40),
			"type":      "customer",
			"state":     "published",
			"body":      "<p>See attached</p>",
			"createdBy": map[string]any{"email": "alice@test.com", "first": "Alice", "last": "S"},
			"customer":  map[string]any{"email": "alice@test.com"},
			"action":    map[string]any{"type": "default", "associatedEntities": map[string]any{}},
		}
		result := cleanThread(m)

		atts, ok := result["attachments"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, atts, 1)
		assert.Equal(t, float64(55), atts[0]["id"])
		assert.Equal(t, "invoice.pdf", atts[0]["filename"])
		assert.Nil(t, atts[0]["_links"])
		assert.Nil(t, atts[0]["state"])
	})
}

func TestCleanCustomer(t *testing.T) {
	raw := json.RawMessage(`{
		"_links": {"self": {"href": "..."}},
		"_embedded": {
			"emails": [{"id": 1, "type": "work", "value": "luke@test.com"}],
			"phones": [],
			"chats": [],
			"properties": [{"name": "Level", "slug": "level", "type": "dropdown"}],
			"social_profiles": [],
			"websites": []
		},
		"id": 100,
		"firstName": "Luke",
		"lastName": "Skywalker",
		"background": "",
		"organization": "Rebel Alliance",
		"gender": "Unknown",
		"draft": false,
		"photoType": "default",
		"photoUrl": "http://photo.jpg",
		"createdAt": "2025-01-01T10:00:00Z"
	}`)

	result := cleanRawObject(raw, cleanCustomer)

	assert.Nil(t, result["_links"])
	assert.Nil(t, result["_embedded"])
	assert.Nil(t, result["background"])      // empty string
	assert.Nil(t, result["gender"])          // "Unknown"
	assert.Nil(t, result["draft"])           // false
	assert.Nil(t, result["photoType"])       // "default"
	assert.Nil(t, result["photoUrl"])        // dropped
	assert.Nil(t, result["phones"])          // empty array hoisted then dropped
	assert.Nil(t, result["chats"])           // empty
	assert.Nil(t, result["websites"])        // empty
	assert.Nil(t, result["social_profiles"]) // empty

	// Hoisted from _embedded
	emails, ok := result["emails"].([]any)
	require.True(t, ok)
	require.Len(t, emails, 1)

	props, ok := result["properties"].([]any)
	require.True(t, ok)
	require.Len(t, props, 1)

	// Kept
	assert.Equal(t, "Luke", result["firstName"])
	assert.Equal(t, "Skywalker", result["lastName"])
	assert.Equal(t, "Rebel Alliance", result["organization"])
}

func TestCleanUser(t *testing.T) {
	m := map[string]any{
		"_links":          map[string]any{"self": "..."},
		"id":              float64(1),
		"firstName":       "Ross",
		"lastName":        "M",
		"email":           "ross@test.com",
		"phone":           "",
		"photoUrl":        "http://photo.jpg",
		"alternateEmails": []any{},
	}
	result := cleanUser(m)

	assert.Nil(t, result["_links"])
	assert.Nil(t, result["photoUrl"])
	assert.Nil(t, result["phone"])
	assert.Nil(t, result["alternateEmails"])
	assert.Equal(t, "Ross", result["firstName"])
}

func TestCleanSavedReply(t *testing.T) {
	m := map[string]any{
		"_links": map[string]any{"self": "..."},
		"id":     float64(1),
		"name":   "Welcome",
		"text":   "<p>Thank you for <strong>reaching out</strong>.</p>",
	}
	result := cleanSavedReply(m)

	assert.Nil(t, result["_links"])
	assert.Contains(t, result["text"], "**reaching out**") // HTML→md
	assert.NotContains(t, result["text"], "<p>")
}

func TestCleanMinimal(t *testing.T) {
	m := map[string]any{
		"_links": map[string]any{"self": "..."},
		"id":     float64(1),
		"name":   "Test",
	}
	result := cleanMinimal(m)

	assert.Nil(t, result["_links"])
	assert.Equal(t, "Test", result["name"])
}

func TestFlattenPersonMap(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"full", map[string]any{"first": "Alice", "last": "Smith", "email": "alice@test.com"}, "Alice Smith (alice@test.com)"},
		{"email only", map[string]any{"email": "alice@test.com"}, "alice@test.com"},
		{"name only", map[string]any{"first": "Alice", "last": "Smith"}, "Alice Smith"},
		{"nil", nil, ""},
		{"not a map", "string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, flattenPersonMap(tt.input))
		})
	}
}
