package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewInventory creates an Inventory with all available tools, resources, and prompts.
// Tools, resources, and prompts are self-describing with their toolset metadata embedded.
// This function is stateless - no dependencies are captured.
// Handlers are generated on-demand during registration via RegisterAll(ctx, server, deps).
// The "default" keyword in WithToolsets will expand to toolsets marked with Default: true.
func NewInventory(t translations.TranslationHelperFunc) *inventory.Builder {
	tools := append(AllTools(t), GovernedActionsTools(t)...)
	tools = append(tools, GovernedActionsExtendedTools(t)...)
	return inventory.NewBuilder().
		SetTools(tools).
		SetResources(AllResources(t)).
		SetPrompts(AllPrompts(t))
}
