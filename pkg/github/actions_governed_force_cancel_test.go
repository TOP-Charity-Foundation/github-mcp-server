package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestGovernedForceCancelWorkflowRunContract(t *testing.T) {
	tool := GovernedForceCancelWorkflowRun(translations.NullTranslationHelper)
	require.Equal(t, "force_cancel_workflow_run", tool.Tool.Name)
	require.NotNil(t, tool.Tool.Annotations)
	require.False(t, tool.Tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Tool.Annotations.DestructiveHint)
	require.True(t, *tool.Tool.Annotations.DestructiveHint)

	schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"owner", "repo", "run_id"}, schema.Required)
	require.Contains(t, schema.Properties, "run_id")
	require.NotContains(t, schema.Properties, "authority")
	require.NotContains(t, schema.Properties, "authorization")
	require.NotContains(t, schema.Properties, "approval")
	require.NotContains(t, schema.Properties, "confirmation")
}

func TestForceCancelWorkflowRunUsesExactGitHubRoute(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/repos/TOP-Charity-Foundation/hyperion-ftap-runtime/actions/runs/34181784501/force-cancel", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	client := gh.NewClient(nil)
	client.BaseURL = baseURL
	client.UploadURL = baseURL

	result, payload, err := forceCancelWorkflowRun(context.Background(), client, "TOP-Charity-Foundation", "hyperion-ftap-runtime", 34181784501)
	require.NoError(t, err)
	require.True(t, called)
	require.NotNil(t, result)
	require.NotNil(t, payload)
}
