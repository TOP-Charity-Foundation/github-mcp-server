package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/stretchr/testify/require"
)

func governedActionsInventoryForTest() []inventory.ServerTool {
	tools := append([]inventory.ServerTool{}, AllTools(stubTranslator)...)
	tools = append(tools, GovernedActionsTools(stubTranslator)...)
	tools = append(tools, GovernedActionsExtendedTools(stubTranslator)...)
	tools = append(tools, GovernedActionsLogsDeploymentTools(stubTranslator)...)
	tools = append(tools, GovernedActionsFinalTools(stubTranslator)...)
	return tools
}

func TestGovernedActionsInventoryIsAdditiveAndUnique(t *testing.T) {
	baseTools := AllTools(stubTranslator)
	allTools := governedActionsInventoryForTest()
	require.Len(t, allTools, len(baseTools)+17, "governed Actions surface must add exactly 17 top-level tools")

	counts := make(map[string]int, len(allTools))
	for _, tool := range allTools {
		counts[tool.Tool.Name]++
	}

	expectedGoverned := []string{
		"list_workflows",
		"get_workflow",
		"dispatch_workflow",
		"resolve_dispatched_workflow_run",
		"list_workflow_runs",
		"get_workflow_run",
		"get_workflow_run_attempt",
		"cancel_workflow_run",
		"rerun_workflow_run",
		"download_workflow_run_logs",
		"download_workflow_run_attempt_logs",
		"list_pending_deployments",
		"list_check_runs_for_ref",
		"list_check_suites_for_ref",
		"review_pending_deployments",
		"enable_workflow",
		"disable_workflow",
	}
	for _, name := range expectedGoverned {
		require.Equalf(t, 1, counts[name], "governed tool %q must be present exactly once", name)
	}

	legacyGrouped := []string{
		"actions_list",
		"actions_get",
		"actions_run_trigger",
		"get_job_logs",
	}
	for _, name := range legacyGrouped {
		require.GreaterOrEqualf(t, counts[name], 1, "legacy grouped tool %q must remain present", name)
	}
}
