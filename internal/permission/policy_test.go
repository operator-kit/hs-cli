package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Empty(t *testing.T) {
	p, err := Parse("")
	require.NoError(t, err)
	assert.True(t, p.IsUnrestricted())
	assert.Equal(t, "unrestricted", p.String())
}

func TestParse_SingleRule(t *testing.T) {
	p, err := Parse("conversations:read")
	require.NoError(t, err)
	assert.False(t, p.IsUnrestricted())
	assert.True(t, p.Allows("conversations", "read"))
	assert.False(t, p.Allows("conversations", "write"))
	assert.False(t, p.Allows("customers", "read"))
}

func TestParse_MultipleRules(t *testing.T) {
	p, err := Parse("conversations:read,customers:read,reports:*")
	require.NoError(t, err)
	assert.True(t, p.Allows("conversations", "read"))
	assert.True(t, p.Allows("customers", "read"))
	assert.True(t, p.Allows("reports", "read"))
	assert.True(t, p.Allows("reports", "write"))
	assert.True(t, p.Allows("reports", "delete"))
	assert.False(t, p.Allows("conversations", "write"))
	assert.False(t, p.Allows("webhooks", "read"))
}

func TestParse_WildcardResource(t *testing.T) {
	p, err := Parse("*:read")
	require.NoError(t, err)
	assert.True(t, p.Allows("conversations", "read"))
	assert.True(t, p.Allows("customers", "read"))
	assert.True(t, p.Allows("anything", "read"))
	assert.False(t, p.Allows("conversations", "write"))
	assert.False(t, p.Allows("conversations", "delete"))
}

func TestParse_FullWildcard(t *testing.T) {
	p, err := Parse("*:*")
	require.NoError(t, err)
	assert.True(t, p.Allows("conversations", "read"))
	assert.True(t, p.Allows("conversations", "write"))
	assert.True(t, p.Allows("conversations", "delete"))
	assert.True(t, p.Allows("anything", "anything-op"))
}

func TestParse_WithWhitespace(t *testing.T) {
	p, err := Parse("  conversations:read , customers:write  ")
	require.NoError(t, err)
	assert.True(t, p.Allows("conversations", "read"))
	assert.True(t, p.Allows("customers", "write"))
}

func TestParse_InvalidFormat(t *testing.T) {
	_, err := Parse("conversations")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected resource:operation")
}

func TestParse_EmptyResource(t *testing.T) {
	_, err := Parse(":read")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty resource")
}

func TestParse_InvalidOperation(t *testing.T) {
	_, err := Parse("conversations:execute")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation must be read|write|delete|*")
}

func TestUnrestrictedPolicy_AllowsEverything(t *testing.T) {
	p := &Policy{}
	assert.True(t, p.IsUnrestricted())
	assert.True(t, p.Allows("anything", "any-op"))
}

func TestPolicy_String(t *testing.T) {
	p, err := Parse("conversations:read,customers:write")
	require.NoError(t, err)
	assert.Equal(t, "conversations:read,customers:write", p.String())
}

func TestPolicy_Rules(t *testing.T) {
	p, err := Parse("conversations:read,customers:*")
	require.NoError(t, err)
	rules := p.Rules()
	assert.Len(t, rules, 2)
	assert.Equal(t, Rule{Resource: "conversations", Operation: "read"}, rules[0])
	assert.Equal(t, Rule{Resource: "customers", Operation: "*"}, rules[1])
}

func TestParse_TrailingComma(t *testing.T) {
	p, err := Parse("conversations:read,")
	require.NoError(t, err)
	assert.True(t, p.Allows("conversations", "read"))
	assert.Len(t, p.Rules(), 1)
}
