package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// CommandHandlerFunc is the function signature for all slash command handlers.
type CommandHandlerFunc func(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse

// commandHandlers maps command keys to their handler functions.
var commandHandlers = map[string]CommandHandlerFunc{
	"plane/create":   handlePlaneCreate,
	"plane/mine":     handlePlaneMine,
	"plane/status":   handlePlaneStatus,
	"connect":        handleConnect,
	"obsidian/setup": handleObsidianSetup,
	"help":           handleHelp,
}

// commandAliases maps short-form command keys to their canonical forms.
var commandAliases = map[string]string{
	"p/c": "plane/create",
	"p/m": "plane/mine",
	"p/s": "plane/status",
}

// ExecuteCommand routes slash commands to the appropriate handler.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)

	// No subcommand provided -- treat as /task help
	if len(split) < 2 {
		return handleHelp(p, c, args, nil), nil
	}

	command := buildCommandKey(split[1:])

	// Check aliases
	if alias, ok := commandAliases[command]; ok {
		command = alias
	}

	if handler, ok := commandHandlers[command]; ok {
		// Pass remaining args after the command key parts
		keyParts := len(strings.Split(command, "/"))
		var subArgs []string
		if len(split) > keyParts+1 {
			subArgs = split[keyParts+1:]
		}
		return handler(p, c, args, subArgs), nil
	}

	return p.suggestCommand(args, split[1:])
}

// buildCommandKey joins command parts with "/" to form a lookup key.
// Example: ["plane", "create"] -> "plane/create"
func buildCommandKey(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	// Build progressively longer keys to find the deepest match
	// For example, ["plane", "create", "foo"] should match "plane/create"
	return strings.Join(parts, "/")
}

// suggestCommand finds the closest matching command and returns a helpful suggestion.
func (p *Plugin) suggestCommand(args *model.CommandArgs, parts []string) (*model.CommandResponse, *model.AppError) {
	input := strings.Join(parts, " ")
	inputKey := strings.Join(parts, "/")

	bestMatch := ""
	bestDistance := len(inputKey) + 1

	// Check all known commands
	allCommands := make([]string, 0, len(commandHandlers)+len(commandAliases))
	for k := range commandHandlers {
		allCommands = append(allCommands, k)
	}
	for k := range commandAliases {
		allCommands = append(allCommands, k)
	}

	for _, cmd := range allCommands {
		d := levenshtein(inputKey, cmd)
		if d < bestDistance {
			bestDistance = d
			bestMatch = cmd
		}
	}

	var message string
	if bestMatch != "" && bestDistance <= 3 {
		suggestion := strings.ReplaceAll(bestMatch, "/", " ")
		message = "Unknown command `/task " + input + "`. Did you mean `/task " + suggestion + "`? Run `/task help` for all commands."
	} else {
		message = "Unknown command `/task " + input + "`. Run `/task help` for all commands."
	}

	return p.respondEphemeral(args, message), nil
}

// levenshtein computes the Levenshtein distance between two strings.
func levenshtein(a, b string) int {
	la := len(a)
	lb := len(b)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows instead of full matrix for space efficiency
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}
