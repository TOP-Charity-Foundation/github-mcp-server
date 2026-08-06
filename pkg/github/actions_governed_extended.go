package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedActionsExtendedTools returns the second additive tranche of
// first-class Actions and Checks operations.
func GovernedActionsExtendedTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		GovernedGetWorkflowRunAttempt(t),
		GovernedEnableWorkflow(t),
		GovernedDisableWorkflow(t),
		GovernedListCheckRunsForRef(t),
		GovernedListCheckSuitesForRef(t),
	}
}

func positiveIntArg(args map[string]any, name string) (int, error) {
	value, err := OptionalIntParam(args, name)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func GovernedGetWorkflowRunAttempt(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "get_workflow_run_attempt", "Get one exact attempt of a GitHub Actions workflow run.", true, false, true,
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
			run, resp, err := client.Actions.GetWorkflowRunAttempt(ctx, owner, repo, int64(runID), attempt, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run attempt", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			data, err := json.Marshal(run)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal workflow run attempt: %w", err)
			}
			return utils.NewToolResultText(string(data)), nil, nil
		})
}

func governedWorkflowStateTool(t translations.TranslationHelperFunc, name, description string, enable bool) inventory.ServerTool {
	return governedActionsTool(t, name, description, false, true, false,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
		}, "workflow_id_or_path"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			workflow, err := RequiredParam[string](args, "workflow_id_or_path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			workflowID, parseErr := strconv.ParseInt(workflow, 10, 64)
			var resp *gh.Response
			if enable {
				if parseErr == nil {
					resp, err = client.Actions.EnableWorkflowByID(ctx, owner, repo, workflowID)
				} else {
					resp, err = client.Actions.EnableWorkflowByFileName(ctx, owner, repo, workflow)
				}
			} else {
				if parseErr == nil {
					resp, err = client.Actions.DisableWorkflowByID(ctx, owner, repo, workflowID)
				} else {
					resp, err = client.Actions.DisableWorkflowByFileName(ctx, owner, repo, workflow)
				}
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to change workflow state", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			return utils.NewToolResultText(fmt.Sprintf("workflow %s completed for %s", name, workflow)), nil, nil
		})
}

func GovernedEnableWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedWorkflowStateTool(t, "enable_workflow", "Enable one exact GitHub Actions workflow.", true)
}

func GovernedDisableWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedWorkflowStateTool(t, "disable_workflow", "Disable one exact GitHub Actions workflow.", false)
}

func GovernedListCheckRunsForRef(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "list_check_runs_for_ref", "List check runs for one exact commit, branch, or tag ref.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"ref":      {Type: "string", Description: "Commit SHA, heads/<branch>, or tags/<tag>"},
			"page":     {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}, "ref"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			result, resp, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check runs for ref", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			data, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal check runs: %w", err)
			}
			return utils.NewToolResultText(string(data)), nil, nil
		})
}

func GovernedListCheckSuitesForRef(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "list_check_suites_for_ref", "List check suites for one exact commit, branch, or tag ref.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"ref":      {Type: "string", Description: "Commit SHA, heads/<branch>, or tags/<tag>"},
			"page":     {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}, "ref"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			result, resp, err := client.Checks.ListCheckSuitesForRef(ctx, owner, repo, ref, &gh.ListCheckSuiteOptions{ListOptions: gh.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check suites for ref", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			data, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal check suites: %w", err)
			}
			return utils.NewToolResultText(string(data)), nil, nil
		})
}
