package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GovernedActionsTools returns additive, first-class GitHub Actions tools.
// The existing grouped actions_list/actions_get/actions_run_trigger tools remain
// registered and unchanged for backwards compatibility.
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
	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   append([]string{"owner", "repo"}, required...),
	}
}

func governedActionsClient(ctx context.Context, deps ToolDependencies, args map[string]any) (string, string, error) {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return "", "", err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return "", "", err
	}
	return owner, repo, nil
}

func GovernedListWorkflows(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        "list_workflows",
		Description: t("TOOL_GOVERNED_LIST_WORKFLOWS_DESCRIPTION", "List GitHub Actions workflows for an explicit repository."),
		Annotations: &mcp.ToolAnnotations{Title: "List workflows", ReadOnlyHint: true},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"page":     {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		result, payload, callErr := listWorkflows(ctx, client, owner, repo, pagination)
		return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelActionsResult), payload, callErr
	})
}

func GovernedGetWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        "get_workflow",
		Description: t("TOOL_GOVERNED_GET_WORKFLOW_DESCRIPTION", "Get one GitHub Actions workflow by exact numeric ID or workflow file path."),
		Annotations: &mcp.ToolAnnotations{Title: "Get workflow", ReadOnlyHint: true},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
		}, "workflow_id_or_path"),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		workflow, err := RequiredParam[string](args, "workflow_id_or_path")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		result, payload, callErr := getWorkflow(ctx, client, owner, repo, workflow)
		return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelActionsResult), payload, callErr
	})
}

func GovernedDispatchWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        "dispatch_workflow",
		Description: t("TOOL_GOVERNED_DISPATCH_WORKFLOW_DESCRIPTION", "Dispatch one workflow on an explicit ref with explicit inputs. No default ref or automatic retry."),
		Annotations: &mcp.ToolAnnotations{Title: "Dispatch workflow", ReadOnlyHint: false},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string", Description: "Exact workflow numeric ID or workflow file path"},
			"ref":                 {Type: "string", Description: "Exact branch or tag reference"},
			"inputs":              {Type: "object", Properties: map[string]*jsonschema.Schema{}},
		}, "workflow_id_or_path", "ref"),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
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
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		return runWorkflow(ctx, client, owner, repo, workflow, ref, inputs)
	})
}

func GovernedListWorkflowRuns(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        "list_workflow_runs",
		Description: t("TOOL_GOVERNED_LIST_WORKFLOW_RUNS_DESCRIPTION", "List workflow runs for an explicit repository, optionally scoped to one workflow."),
		Annotations: &mcp.ToolAnnotations{Title: "List workflow runs", ReadOnlyHint: true},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"workflow_id_or_path": {Type: "string"},
			"workflow_runs_filter": {Type: "object", Properties: map[string]*jsonschema.Schema{
				"actor":  {Type: "string"},
				"branch": {Type: "string"},
				"event":  {Type: "string"},
				"status": {Type: "string"},
			}},
			"page":     {Type: "number", Minimum: jsonschema.Ptr(1.0)},
			"per_page": {Type: "number", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(100.0)},
		}),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		workflow, err := OptionalParam[string](args, "workflow_id_or_path")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		result, payload, callErr := listWorkflowRuns(ctx, client, args, owner, repo, workflow, pagination)
		return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelActionsResult), payload, callErr
	})
}

func GovernedGetWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        "get_workflow_run",
		Description: t("TOOL_GOVERNED_GET_WORKFLOW_RUN_DESCRIPTION", "Get one exact GitHub Actions workflow run."),
		Annotations: &mcp.ToolAnnotations{Title: "Get workflow run", ReadOnlyHint: true},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
		}, "run_id"),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		runID, err := OptionalIntParam(args, "run_id")
		if err != nil || runID <= 0 {
			return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		result, payload, callErr := getWorkflowRun(ctx, client, owner, repo, int64(runID))
		return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelActionsResult), payload, callErr
	})
}

func GovernedCancelWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedRunMutationTool(t, "cancel_workflow_run", "Cancel one exact queued or in-progress workflow run.", true, cancelWorkflowRun)
}

func GovernedRerunWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	return governedRunMutationTool(t, "rerun_workflow_run", "Create one full rerun attempt for an exact workflow run.", false, rerunWorkflowRun)
}

type governedRunMutation func(context.Context, interfaceClient, string, string, int64) (*mcp.CallToolResult, any, error)

// interfaceClient is intentionally avoided in concrete handlers; this alias is
// replaced by the adapter below to keep the public factory concise.
type interfaceClient = *githubClientAlias

type githubClientAlias = struct{}

func governedRunMutationTool(t translations.TranslationHelperFunc, name, description string, destructive bool, _ any) inventory.ServerTool {
	return NewTool(ToolsetMetadataActions, mcp.Tool{
		Name:        name,
		Description: t("TOOL_GOVERNED_"+name+"_DESCRIPTION", description),
		Annotations: &mcp.ToolAnnotations{Title: name, ReadOnlyHint: false, DestructiveHint: jsonschema.Ptr(destructive)},
		InputSchema: governedRepoSchema(map[string]*jsonschema.Schema{
			"run_id": {Type: "number", Minimum: jsonschema.Ptr(1.0)},
		}, "run_id"),
	}, []scopes.Scope{scopes.Repo}, func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, repo, err := governedActionsClient(ctx, deps, args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		runID, err := OptionalIntParam(args, "run_id")
		if err != nil || runID <= 0 {
			return utils.NewToolResultError("run_id must be a positive integer"), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		switch name {
		case "cancel_workflow_run":
			return cancelWorkflowRun(ctx, client, owner, repo, int64(runID))
		case "rerun_workflow_run":
			return rerunWorkflowRun(ctx, client, owner, repo, int64(runID))
		default:
			return utils.NewToolResultError("unsupported governed run mutation"), nil, nil
		}
	})
}
