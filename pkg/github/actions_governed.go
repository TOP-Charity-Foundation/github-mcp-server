package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedActionsTools returns additive first-class Actions tools. The existing
// grouped actions_list, actions_get, and actions_run_trigger tools remain intact.
func GovernedActionsTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		GovernedListWorkflows(t),
		GovernedGetWorkflow(t),
		GovernedDispatchWorkflow(t),
		GovernedListWorkflowRuns(t),
		GovernedGetWorkflowRun(t),
		GovernedCancelWorkflowRun(t),
		GovernedRerunWorkflowRun(t),
	}
}

func governedRepoSchema(extra map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	properties := map[string]*jsonschema.Schema{
		"owner": {Type: "string", Description: DescriptionRepositoryOwner},
		"repo":  {Type: "string", Description: DescriptionRepositoryName},
	}
	for name, schema := range extra {
		properties[name] = schema
	}
	return &jsonschema.Schema{Type: "object", Properties: properties, Required: append([]string{"owner", "repo"}, required...)}
}

type governedActionsHandler func(context.Context, *gh.Client, string, string, map[string]any) (*mcp.CallToolResult, any, error)

func governedActionsTool(t translations.TranslationHelperFunc, name, description string, readOnly, destructive, attachIFC bool, schema *jsonschema.Schema, handler governedActionsHandler) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        name,
		Description: t("TOOL_GOVERNED_"+name+"_DESCRIPTION", description),
		Annotations: &mcp.ToolAnnotations{
			Title:           name,
			ReadOnlyHint:    readOnly,
			DestructiveHint: jsonschema.Ptr(destructive),
		},
		InputSchema: schema,
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		result, payload, callErr := handler(ctx, client, owner, repo, args)
		if attachIFC {
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelActionsResult)
		}
		return result, payload, callErr
	})
}

func GovernedListWorkflows(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "list_workflows", "List GitHub Actions workflows for an explicit repository.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"page":     {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			return listWorkflows(ctx, client, owner, repo, pagination)
		})
}

func GovernedGetWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "get_workflow", "Get one workflow by exact numeric ID or workflow file path.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
		}, "workflow_id_or_path"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			workflow, err := RequiredParam[string](args, "workflow_id_or_path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			return getWorkflow(ctx, client, owner, repo, workflow)
		})
}

func GovernedDispatchWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "dispatch_workflow", "Dispatch one workflow on an explicit ref with explicit inputs. No default ref or automatic retry.", false, false, false,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
			"ref":                 {Type: "string", Description: "Exact branch or tag reference"},
			"inputs":              {Type: "object", Properties: map[string]*jsonschema.Schema{}},
		}, "workflow_id_or_path", "ref"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			workflow, err := RequiredParam[string](args, "workflow_id_or_path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			inputs, err := OptionalParam[map[string]any](args, "inputs")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			return runWorkflow(ctx, client, owner, repo, workflow, ref, inputs)
		})
}

func GovernedListWorkflowRuns(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "list_workflow_runs", "List workflow runs for an explicit repository, optionally scoped to one workflow.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string"},
			"workflow_runs_filter": {Type: "object", Properties: map[string]*jsonschema.Schema{
				"actor": {Type: "string"}, "branch": {Type: "string"}, "event": {Type: "string"}, "status": {Type: "string"},
			}},
			"page": {Type: "number", Minimum: jsonschema.Ptr(1.0)}, "per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			workflow, err := OptionalParam[string](args, "workflow_id_or_path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			return listWorkflowRuns(ctx, client, args, owner, repo, workflow, pagination)
		})
}

func GovernedGetWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "get_workflow_run", "Get one exact GitHub Actions workflow run.", true, false, true,
		governedRepoSchema(map[string]*jsonschema.Schema{"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)}}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil || runID <= 0 {
				return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
			}
			return getWorkflowRun(ctx, client, owner, repo, int64(runID))
		})
}

func GovernedCancelWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "cancel_workflow_run", "Cancel one exact queued or in-progress workflow run.", false, true, false,
		governedRepoSchema(map[string]*jsonschema.Schema{"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)}}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil || runID <= 0 {
				return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
			}
			return cancelWorkflowRun(ctx, client, owner, repo, int64(runID))
		})
}

func GovernedRerunWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedActionsTool(t, "rerun_workflow_run", "Create one full rerun attempt for an exact workflow run.", false, false, false,
		governedRepoSchema(map[string]*jsonschema.Schema{"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)}}, "run_id"),
		func(ctx context.Context, client *gh.Client, owner, repo string, args map[string]any) (*mcp.CallToolResult, any, error) {
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil || runID <= 0 {
				return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
			}
			return rerunWorkflowRun(ctx, client, owner, repo, int64(runID))
		})
}
