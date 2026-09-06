package github

import (
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

var governedActionsReadOnlyExpectations = map[string]bool{
	"list_workflows":                     true,
	"get_workflow":                       true,
	"dispatch_workflow":                  false,
	"resolve_dispatched_workflow_run":    true,
	"list_workflow_runs":                 true,
	"get_workflow_run":                   true,
	"get_workflow_run_attempt":           true,
	"cancel_workflow_run":                false,
	"rerun_workflow_run":                 false,
	"download_workflow_run_logs":         true,
	"download_workflow_run_attempt_logs": true,
	"list_pending_deployments":           true,
	"list_check_runs_for_ref":            true,
	"list_check_suites_for_ref":          true,
	"review_pending_deployments":         false,
	"enable_workflow":                    false,
	"disable_workflow":                   false,
}

var governedActionsDestructiveExpectations = map[string]bool{
	"dispatch_workflow":           false,
	"cancel_workflow_run":         true,
	"rerun_workflow_run":          false,
	"review_pending_deployments":  true,
	"enable_workflow":             true,
	"disable_workflow":            true,
}

var forbiddenApprovalSchemaProperties = map[string]struct{}{
	"authority":              {},
	"authorityMode":          {},
	"authority_mode":         {},
	"confirmation":           {},
	"fullAuthority":          {},
	"fullAuthorityRelease":   {},
	"full_authority":         {},
	"full_authority_release": {},
}

var forbiddenApprovalDescriptionPhrases = []string{
	"authorized_full_authority_github_mcp_write",
	"authority envelope",
	"full authority",
}

func TestGovernedActionsDoNotExposeApprovalEnvelopeInputs(t *testing.T) {
	for _, tool := range governedActionsInventoryForTest() {
		if _, governed := governedActionsReadOnlyExpectations[tool.Tool.Name]; !governed {
			continue
		}
		t.Run(tool.Tool.Name, func(t *testing.T) {
			assertNoForbiddenApprovalDescription(t, tool.Tool.Name+" description", tool.Tool.Description)

			schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
			require.True(t, ok, "governed Actions tool %q must use jsonschema.Schema input", tool.Tool.Name)
			assertNoForbiddenApprovalSchemaFields(t, tool.Tool.Name, schema)
		})
	}
}

func TestGovernedActionsReadOnlyAndDestructiveHintsMatchBehavior(t *testing.T) {
	found := make(map[string]bool, len(governedActionsReadOnlyExpectations))
	for _, tool := range governedActionsInventoryForTest() {
		expectedReadOnly, governed := governedActionsReadOnlyExpectations[tool.Tool.Name]
		if !governed {
			continue
		}
		found[tool.Tool.Name] = true
		t.Run(tool.Tool.Name, func(t *testing.T) {
			require.NotNil(t, tool.Tool.Annotations, "governed Actions tool %q must declare annotations", tool.Tool.Name)
			require.Equal(t, expectedReadOnly, tool.Tool.Annotations.ReadOnlyHint, "governed Actions readOnlyHint must match behavior")

			if expectedDestructive, ok := governedActionsDestructiveExpectations[tool.Tool.Name]; ok {
				require.NotNil(t, tool.Tool.Annotations.DestructiveHint, "write-capable governed Actions tool %q must declare destructiveHint", tool.Tool.Name)
				require.Equal(t, expectedDestructive, *tool.Tool.Annotations.DestructiveHint, "governed Actions destructiveHint must match behavior")
			}
		})
	}

	for name := range governedActionsReadOnlyExpectations {
		require.Truef(t, found[name], "expected governed Actions tool %q to be registered", name)
	}
}

func assertNoForbiddenApprovalSchemaFields(t *testing.T, path string, schema *jsonschema.Schema) {
	t.Helper()
	if schema == nil {
		return
	}
	assertNoForbiddenApprovalDescription(t, path+" schema description", schema.Description)
	for _, required := range schema.Required {
		if _, blocked := forbiddenApprovalSchemaProperties[required]; blocked {
			t.Fatalf("%s requires blocked approval-envelope input %q", path, required)
		}
	}
	for name, child := range schema.Properties {
		if _, blocked := forbiddenApprovalSchemaProperties[name]; blocked {
			t.Fatalf("%s exposes blocked approval-envelope input %q", path, name)
		}
		assertNoForbiddenApprovalSchemaFields(t, path+"."+name, child)
	}
	assertNoForbiddenApprovalSchemaFields(t, path+"[]", schema.Items)
}

func assertNoForbiddenApprovalDescription(t *testing.T, path string, value string) {
	t.Helper()
	lower := strings.ToLower(value)
	for _, phrase := range forbiddenApprovalDescriptionPhrases {
		if strings.Contains(lower, phrase) {
			t.Fatalf("%s contains blocked approval-envelope phrase %q", path, phrase)
		}
	}
}
