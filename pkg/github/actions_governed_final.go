package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedActionsFinalTools returns the final Work Order operations.
func GovernedActionsFinalTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		GovernedReviewPendingDeployments(t),
		GovernedResolveDispatchedWorkflowRun(t),
	}
}

func GovernedReviewPendingDeployments(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "review_pending_deployments", "Approve or reject explicitly identified pending deployment environments for one workflow run.", false, true, false,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"environment_ids": {
				Type:        "array",
				Description: "One or more exact pending deployment environment IDs",
				Items:       &jsonschema.Schema{Type: "number", Minimum: jsonschema.Ptr(1.0)},
				MinItems:    jsonschema.Ptr(uint64(1)),
			},
			"state": {
				Type:        "string",
				Description: "Review decision",
				Enum:        []any{"approved", "rejected"},
			},
			"comment": {Type: "string", Description: "Required audit comment"},
		}, "run_id", "environment_ids", "state", "comment"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil || runID <= 0 {
				return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
			}
			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state = strings.ToLower(strings.TrimSpace(state))
			if state != "approved" && state != "rejected" {
				return utils.NewToolResultError("state must be exactly approved or rejected"), nil, nil
			}
			comment, err := RequiredParam[string](args, "comment")
			if err != nil || strings.TrimSpace(comment) == "" {
				return utils.NewToolResultError("comment is required and must not be empty"), nil, nil
			}

			rawIDs, ok := args["environment_ids"].([]any)
			if !ok || len(rawIDs) == 0 {
				return utils.NewToolResultError("environment_ids must contain at least one positive integer"), nil, nil
			}
			environmentIDs := make([]int64, 0, len(rawIDs))
			seen := make(map[int64]struct{}, len(rawIDs))
			for _, rawID := range rawIDs {
				var id int64
				switch value := rawID.(type) {
				case float64:
					id = int64(value)
					if float64(id) != value {
						return utils.NewToolResultError("environment_ids must contain integers only"), nil, nil
					}
				case int:
					id = int64(value)
				case int64:
					id = value
				default:
					return utils.NewToolResultError("environment_ids must contain integers only"), nil, nil
				}
				if id <= 0 {
					return utils.NewToolResultError("environment_ids must contain positive integers only"), nil, nil
				}
				if _, duplicate := seen[id]; duplicate {
					return utils.NewToolResultError("environment_ids must not contain duplicates"), nil, nil
				}
				seen[id] = struct{}{}
				environmentIDs = append(environmentIDs, id)
			}

			deployments, resp, err := client.Actions.PendingDeployments(ctx, owner, repo, int64(runID), &gh.PendingDeploymentsRequest{
				EnvironmentIDs: environmentIDs,
				State:          state,
				Comment:        strings.TrimSpace(comment),
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to review pending deployments", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			encoded, err := json.Marshal(deployments)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal reviewed deployments: %w", err)
			}
			return utils.NewToolResultText(string(encoded)), nil, nil
		})
}

func GovernedResolveDispatchedWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "resolve_dispatched_workflow_run", "Resolve exactly one workflow_dispatch run using workflow, ref, dispatch timestamp, and optional head SHA. Ambiguous matches fail closed.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
			"ref":                 {Type: "string", Description: "Exact branch or tag used for dispatch"},
			"dispatched_after":    {Type: "string", Description: "RFC3339 timestamp captured immediately before dispatch"},
			"head_sha":            {Type: "string", Description: "Optional exact commit SHA expected for the dispatched run"},
			"window_seconds": {
				Type:        "number",
				Description: "Bounded resolution window after dispatched_after; defaults to 120 seconds",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(600.0),
			},
		}, "workflow_id_or_path", "ref", "dispatched_after"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			workflow, err := RequiredParam[string](args, "workflow_id_or_path")
			if err != nil || strings.TrimSpace(workflow) == "" {
				return utils.NewToolResultError("workflow_id_or_path is required"), nil, nil
			}
			ref, err := RequiredParam[string](args, "ref")
			if err != nil || strings.TrimSpace(ref) == "" {
				return utils.NewToolResultError("ref is required"), nil, nil
			}
			dispatchedAfterRaw, err := RequiredParam[string](args, "dispatched_after")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			dispatchedAfter, err := time.Parse(time.RFC3339, dispatchedAfterRaw)
			if err != nil {
				return utils.NewToolResultError("dispatched_after must be an RFC3339 timestamp"), nil, nil
			}
			headSHA, err := OptionalParam[string](args, "head_sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			windowSeconds, err := OptionalIntParam(args, "window_seconds")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if windowSeconds == 0 {
				windowSeconds = 120
			}
			if windowSeconds < 1 || windowSeconds > 600 {
				return utils.NewToolResultError("window_seconds must be between 1 and 600"), nil, nil
			}
			windowEnd := dispatchedAfter.Add(time.Duration(windowSeconds) * time.Second)

			opts := &gh.ListWorkflowRunsOptions{
				Branch:  strings.TrimSpace(ref),
				Event:   "workflow_dispatch",
				Created: ">=" + dispatchedAfter.UTC().Format(time.RFC3339),
				HeadSHA: strings.TrimSpace(headSHA),
				ListOptions: gh.ListOptions{
					PerPage: 100,
					Page:    1,
				},
			}

			var runs *gh.WorkflowRuns
			var resp *gh.Response
			workflow = strings.TrimSpace(workflow)
			if workflowID, parseErr := strconv.ParseInt(workflow, 10, 64); parseErr == nil {
				runs, resp, err = client.Actions.ListWorkflowRunsByID(ctx, owner, repo, workflowID, opts)
			} else {
				runs, resp, err = client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflow, opts)
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to resolve dispatched workflow run", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}

			matches := make([]*gh.WorkflowRun, 0, 1)
			for _, run := range runs.WorkflowRuns {
				if run == nil || run.CreatedAt == nil {
					continue
				}
				created := run.CreatedAt.Time
				if created.Before(dispatchedAfter) || created.After(windowEnd) {
					continue
				}
				if run.GetEvent() != "workflow_dispatch" || run.GetHeadBranch() != strings.TrimSpace(ref) {
					continue
				}
				if strings.TrimSpace(headSHA) != "" && run.GetHeadSHA() != strings.TrimSpace(headSHA) {
					continue
				}
				matches = append(matches, run)
			}

			if len(matches) == 0 {
				return utils.NewToolResultError("no workflow_dispatch run matched the supplied workflow, ref, timestamp window, and head SHA constraints"), nil, nil
			}
			if len(matches) > 1 {
				return utils.NewToolResultError(fmt.Sprintf("dispatch resolution is ambiguous: %d runs matched; provide head_sha or a narrower window", len(matches))), nil, nil
			}
			encoded, err := json.Marshal(matches[0])
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal resolved workflow run: %w", err)
			}
			return utils.NewToolResultText(string(encoded)), nil, nil
		})
}
