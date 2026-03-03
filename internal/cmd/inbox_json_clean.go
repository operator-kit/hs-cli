package cmd

import (
	"encoding/json"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// cleanRawItems applies cleanFn to each raw JSON item in a slice.
func cleanRawItems(items []json.RawMessage, cleanFn func(map[string]any) map[string]any) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, raw := range items {
		var m map[string]any
		json.Unmarshal(raw, &m)
		out[i] = cleanFn(m)
	}
	return out
}

// cleanRawObject applies cleanFn to a single raw JSON object.
func cleanRawObject(data json.RawMessage, cleanFn func(map[string]any) map[string]any) map[string]any {
	var m map[string]any
	json.Unmarshal(data, &m)
	return cleanFn(m)
}

// dropKeys removes specified keys from a map.
func dropKeys(m map[string]any, keys ...string) {
	for _, k := range keys {
		delete(m, k)
	}
}

// dropEmptyArrays removes keys whose value is an empty JSON array.
func dropEmptyArrays(m map[string]any) {
	for k, v := range m {
		if arr, ok := v.([]any); ok && len(arr) == 0 {
			delete(m, k)
		}
	}
}

// dropEmptyStrings removes keys whose value is an empty string.
func dropEmptyStrings(m map[string]any) {
	for k, v := range m {
		if s, ok := v.(string); ok && s == "" {
			delete(m, k)
		}
	}
}

// flattenPersonMap converts a person object {first, last, email, ...} to "Name (email)".
func flattenPersonMap(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	first, _ := m["first"].(string)
	last, _ := m["last"].(string)
	email, _ := m["email"].(string)
	name := strings.TrimSpace(first + " " + last)
	if email != "" {
		if name != "" {
			return name + " (" + email + ")"
		}
		return email
	}
	return name
}

// htmlToMd converts HTML to markdown, falling back to stripped HTML on error.
func htmlToMd(html string) string {
	if html == "" {
		return ""
	}
	md, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return stripHTMLTags(html)
	}
	return strings.TrimSpace(md)
}

// hoistEmbedded moves non-empty arrays from _embedded to the parent and drops the wrapper.
func hoistEmbedded(m map[string]any) {
	emb, ok := m["_embedded"].(map[string]any)
	if !ok {
		delete(m, "_embedded")
		return
	}
	for k, v := range emb {
		if arr, ok := v.([]any); ok && len(arr) > 0 {
			m[k] = arr
		}
	}
	delete(m, "_embedded")
}

// cleanAttachments strips _links, state, width, height from attachment objects.
func cleanAttachments(atts []any) []map[string]any {
	out := make([]map[string]any, 0, len(atts))
	for _, a := range atts {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}
		clean := map[string]any{
			"id":       am["id"],
			"filename": am["filename"],
			"mimeType": am["mimeType"],
			"size":     am["size"],
		}
		out = append(out, clean)
	}
	return out
}

// isSentinelPerson returns true if a person object is a sentinel (id=0, email="unknown").
func isSentinelPerson(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return true
	}
	if id, ok := m["id"].(float64); ok && id == 0 {
		return true
	}
	if email, ok := m["email"].(string); ok && email == "unknown" {
		return true
	}
	return false
}

// isDefaultAction returns true if action is the noise default {type: "default", associatedEntities: {}}.
func isDefaultAction(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return true
	}
	t, _ := m["type"].(string)
	return t == "default" && getActionText(m) == ""
}

func getActionText(action map[string]any) string {
	s, _ := action["text"].(string)
	return s
}

// --- Per-resource clean functions ---

// cleanConversation transforms a raw conversation object to clean JSON.
func cleanConversation(m map[string]any) map[string]any {
	dropKeys(m, "_links")

	// Hoist embedded threads if present (non-empty)
	if emb, ok := m["_embedded"].(map[string]any); ok {
		if threads, ok := emb["threads"].([]any); ok && len(threads) > 0 {
			cleaned := make([]map[string]any, 0, len(threads))
			for _, t := range threads {
				if tm, ok := t.(map[string]any); ok {
					cleaned = append(cleaned, cleanThread(tm))
				}
			}
			m["threads"] = cleaned
		}
	}
	delete(m, "_embedded")

	// state: drop when "published", keep for drafts
	if s, _ := m["state"].(string); s == "published" {
		delete(m, "state")
	}

	// Sentinel closedBy/closedByUser
	if id, _ := m["closedBy"].(float64); id == 0 {
		delete(m, "closedBy")
	}
	if isSentinelPerson(m["closedByUser"]) {
		delete(m, "closedByUser")
	} else {
		m["closedByUser"] = flattenPersonMap(m["closedByUser"])
	}

	// Flatten persons
	m["customer"] = flattenPersonMap(m["primaryCustomer"])
	delete(m, "primaryCustomer")
	delete(m, "createdBy") // duplicate of primaryCustomer

	if m["assignee"] != nil {
		m["assignee"] = flattenPersonMap(m["assignee"])
	}

	// customerWaitingSince: keep ISO time only
	if cws, ok := m["customerWaitingSince"].(map[string]any); ok {
		if t, ok := cws["time"].(string); ok {
			m["customerWaitingSince"] = t
		}
	}

	// Rename fields
	if v, ok := m["userUpdatedAt"]; ok {
		m["updatedAt"] = v
		delete(m, "userUpdatedAt")
	}
	if v, ok := m["threads"]; ok {
		// Only rename the int thread count, not the hoisted thread array
		if _, isFloat := v.(float64); isFloat {
			m["threadCount"] = v
			delete(m, "threads")
		}
	}

	// Drop photoUrl from any remaining person objects
	dropKeys(m, "photoUrl")

	// Drop empty arrays/strings
	dropEmptyArrays(m)
	dropEmptyStrings(m)

	return m
}

// cleanThread transforms a raw thread object to clean JSON.
func cleanThread(m map[string]any) map[string]any {
	dropKeys(m, "_links", "customer") // customer is duplicate of createdBy

	// Hoist attachments from _embedded
	if emb, ok := m["_embedded"].(map[string]any); ok {
		if atts, ok := emb["attachments"].([]any); ok && len(atts) > 0 {
			m["attachments"] = cleanAttachments(atts)
		}
	}
	delete(m, "_embedded")

	// state: keep for drafts, drop for published
	if s, _ := m["state"].(string); s == "published" || s == "" {
		delete(m, "state")
	}

	// Flatten createdBy → from
	if cb := m["createdBy"]; cb != nil {
		m["from"] = flattenPersonMap(cb)
		delete(m, "createdBy")
	}

	// Flatten assignedTo if present
	if at := m["assignedTo"]; at != nil {
		m["assignedTo"] = flattenPersonMap(at)
	}

	// Type-specific handling
	threadType, _ := m["type"].(string)

	if threadType == "lineitem" {
		// Lineitems: action.text IS the content
		if action, ok := m["action"].(map[string]any); ok {
			text := getActionText(action)
			if text != "" {
				m["action"] = text
			} else {
				delete(m, "action")
			}
		}
		// Lineitems have no body, drop noise fields
		delete(m, "source")
	} else {
		// Non-lineitem: handle action
		if isDefaultAction(m["action"]) {
			delete(m, "action")
		}
		// HTML → markdown body
		if body, ok := m["body"].(string); ok {
			m["body"] = htmlToMd(body)
		}
	}

	dropEmptyArrays(m)
	dropEmptyStrings(m)
	dropKeys(m, "photoUrl")

	return m
}

// cleanCustomer transforms a raw customer object to clean JSON.
func cleanCustomer(m map[string]any) map[string]any {
	dropKeys(m, "_links")

	// Hoist non-empty embedded sub-resources
	hoistEmbedded(m)

	// Drop noise defaults
	if g, _ := m["gender"].(string); g == "Unknown" || g == "" {
		delete(m, "gender")
	}
	if d, ok := m["draft"].(bool); ok && !d {
		delete(m, "draft")
	}
	if pt, _ := m["photoType"].(string); pt == "default" || pt == "" {
		delete(m, "photoType")
	}
	dropKeys(m, "photoUrl")

	dropEmptyArrays(m)
	dropEmptyStrings(m)

	return m
}

// cleanUser transforms a raw user object to clean JSON.
func cleanUser(m map[string]any) map[string]any {
	dropKeys(m, "_links", "photoUrl")
	dropEmptyArrays(m)
	dropEmptyStrings(m)
	return m
}

// cleanMinimal drops only _links (for resources needing minimal cleanup).
func cleanMinimal(m map[string]any) map[string]any {
	dropKeys(m, "_links")
	return m
}

// cleanOrganization transforms a raw organization object.
func cleanOrganization(m map[string]any) map[string]any {
	dropKeys(m, "_links")
	dropEmptyArrays(m)
	return m
}

// cleanSavedReply transforms a raw saved reply object.
func cleanSavedReply(m map[string]any) map[string]any {
	dropKeys(m, "_links")
	if text, ok := m["text"].(string); ok {
		m["text"] = htmlToMd(text)
	}
	return m
}

// cleanAttachmentItem transforms a raw attachment list item.
func cleanAttachmentItem(m map[string]any) map[string]any {
	dropKeys(m, "_links", "state", "width", "height")
	return m
}
