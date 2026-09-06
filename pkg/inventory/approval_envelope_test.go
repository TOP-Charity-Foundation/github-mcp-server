package inventory

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestSanitizeApprovalEnvelopeParamsRemovesModelSuppliedApprovalFields(t *testing.T) {
	tool := mcp.Tool{
		Name: "write_tool",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":                {Type: "string"},
				"repo":                 {Type: "string"},
				"authority":            {Type: "string"},
				"confirmation":         {Type: "string"},
				"fullAuthority":        {Type: "string"},
				"fullAuthorityRelease": {Type: "string"},
				"state":                {Type: "string"},
			},
			Required: []string{"owner", "repo", "authority", "confirmation", "fullAuthority", "state"},
		},
	}

	SanitizeApprovalEnvelopeParams(&tool)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	for _, blocked := range []string{"authority", "confirmation", "fullAuthority", "fullAuthorityRelease"} {
		_, exists := schema.Properties[blocked]
		require.Falsef(t, exists, "blocked approval-envelope property %q must be removed", blocked)
		require.NotContains(t, schema.Required, blocked)
	}
	require.Contains(t, schema.Properties, "owner")
	require.Contains(t, schema.Properties, "repo")
	require.Contains(t, schema.Properties, "state")
	require.ElementsMatch(t, []string{"owner", "repo", "state"}, schema.Required)
}

func TestSanitizeApprovalEnvelopeParamsDoesNotMutateOriginalSchema(t *testing.T) {
	original := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"authority": {Type: "string"},
			"safe":      {Type: "string"},
		},
		Required: []string{"authority", "safe"},
	}
	tool := mcp.Tool{Name: "write_tool", InputSchema: original}

	SanitizeApprovalEnvelopeParams(&tool)

	_, originalStillHasAuthority := original.Properties["authority"]
	require.True(t, originalStillHasAuthority, "sanitization must not mutate the shared source schema")
	require.Contains(t, original.Required, "authority")

	sanitized, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	_, sanitizedHasAuthority := sanitized.Properties["authority"]
	require.False(t, sanitizedHasAuthority)
	require.Contains(t, sanitized.Properties, "safe")
	require.ElementsMatch(t, []string{"safe"}, sanitized.Required)
}
