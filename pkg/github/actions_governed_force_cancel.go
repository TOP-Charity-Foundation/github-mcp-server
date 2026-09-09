package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedForceCancelWorkflowRun force-cancels one exact workflow run through
// GitHub's dedicated force-cancel endpoint. This is intentionally separate from
// normal cancellation and is intended only for runs that do not respond to the
// standard cancel operation.
func GovernedForceCancelWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "force_cancel_workflow_run", "Force cancel one exact workflow run that did not respond to normal cancellation.", false, true, false,
		governedRepoSchema(map[string]*jsonschema.Schema{"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)}}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil || runID <= 0 {
				return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
			}
			return forceCancelWorkflowRun(ctx, client, owner, repo, int64(runID))
		})
}

func forceCancelWorkflowRun(ctx context.Context, client *gh.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	route := fmt.Sprintf("repos/%v/%v/actions/runs/%v/force-cancel", owner, repo, runID)
	req, err := client.NewRequest(ctx, http.MethodPost, route, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create force-cancel workflow request: %w", err)
	}

	resp, err := client.Do(ctx, req, nil)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to force cancel workflow run", resp, err), nil, nil
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	payload := map[string]any{
		"message": "Force cancel request accepted",
		"run_id":  runID,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal force-cancel response: %w", err)
	}
	return utils.NewToolResultText(string(encoded)), payload, nil
}
