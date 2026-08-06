package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedActionsLogsDeploymentTools returns additive read-only log and
// deployment-inspection operations.
func GovernedActionsLogsDeploymentTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		GovernedDownloadWorkflowRunLogs(t),
		GovernedDownloadWorkflowRunAttemptLogs(t),
		GovernedListPendingDeployments(t),
	}
}

func GovernedDownloadWorkflowRunLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "download_workflow_run_logs", "Return the short-lived GitHub download URL for one workflow run log archive.", true, false, false,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
		}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := positiveIntArg(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			logURL, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, int64(runID), 0)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run logs", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if logURL == nil {
				return utils.NewToolResultError("GitHub returned no workflow run log URL"), nil, nil
			}
			return utils.NewToolResultText(logURL.String()), nil, nil
		})
}

func GovernedDownloadWorkflowRunAttemptLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "download_workflow_run_attempt_logs", "Return the short-lived GitHub download URL for one exact workflow run attempt log archive.", true, false, false,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id":         {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"attempt_number": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
		}, "run_id", "attempt_number"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := positiveIntArg(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			attempt, err := positiveIntArg(args, "attempt_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			logURL, resp, err := client.Actions.GetWorkflowRunAttemptLogs(ctx, owner, repo, int64(runID), attempt, 0)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run attempt logs", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if logURL == nil {
				return utils.NewToolResultError("GitHub returned no workflow run attempt log URL"), nil, nil
			}
			return utils.NewToolResultText(logURL.String()), nil, nil
		})
}

func GovernedListPendingDeployments(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "list_pending_deployments", "List deployment environments awaiting protection-rule approval for one workflow run.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
		}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := positiveIntArg(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			deployments, resp, err := client.Actions.GetPendingDeployments(ctx, owner, repo, int64(runID))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list pending deployments", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			data, err := json.Marshal(deployments)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal pending deployments: %w", err)
			}
			return utils.NewToolResultText(string(data)), nil, nil
		})
}
