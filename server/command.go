package main

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// registerCommands registers the /task slash command with full autocomplete tree.
func (p *Plugin) registerCommands() error {
	// Root command
	task := model.NewAutocompleteData("task", "[command]", "Task management commands")

	// /task plane subcommands
	plane := model.NewAutocompleteData("plane", "[subcommand]", "Plane task management")
	create := model.NewAutocompleteData("create", "[title]", "Create a new task in Plane")
	create.AddTextArgument("Quick create with title", "[title]", "")
	mine := model.NewAutocompleteData("mine", "", "Show your assigned tasks in Plane")
	status := model.NewAutocompleteData("status", "[project]", "Show project status summary")
	status.AddTextArgument("Project name or identifier", "[project]", "")
	plane.AddCommand(create)
	plane.AddCommand(mine)
	plane.AddCommand(status)

	// /task p alias subcommands (mirrors plane with short names)
	planeAlias := model.NewAutocompleteData("p", "[subcommand]", "Plane (alias)")
	createAlias := model.NewAutocompleteData("c", "[title]", "Create task (alias)")
	createAlias.AddTextArgument("Quick create with title", "[title]", "")
	mineAlias := model.NewAutocompleteData("m", "", "Your tasks (alias)")
	statusAlias := model.NewAutocompleteData("s", "[project]", "Project status (alias)")
	statusAlias.AddTextArgument("Project name or identifier", "[project]", "")
	planeAlias.AddCommand(createAlias)
	planeAlias.AddCommand(mineAlias)
	planeAlias.AddCommand(statusAlias)

	// /task connect
	connect := model.NewAutocompleteData("connect", "", "Link your Mattermost account with Plane")

	// /task obsidian subcommands
	obsidian := model.NewAutocompleteData("obsidian", "[subcommand]", "Obsidian integration")
	obsidianSetup := model.NewAutocompleteData("setup", "", "Configure Obsidian REST API endpoint")
	obsidian.AddCommand(obsidianSetup)

	// /task help
	help := model.NewAutocompleteData("help", "", "Show available commands and usage")

	// Build tree
	task.AddCommand(plane)
	task.AddCommand(planeAlias)
	task.AddCommand(connect)
	task.AddCommand(obsidian)
	task.AddCommand(help)

	return p.API.RegisterCommand(&model.Command{
		Trigger:          "task",
		DisplayName:      "Task Management",
		Description:      "Create and manage tasks in Plane and Obsidian",
		AutoComplete:     true,
		AutoCompleteDesc: "Task management commands",
		AutocompleteData: task,
	})
}
