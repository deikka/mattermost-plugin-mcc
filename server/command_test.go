package main

import "testing"

func TestHelpCommand(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-01 — verify /task help returns formatted command list")
}

func TestCommandRouting(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-01 — verify handler map dispatches correctly")
}

func TestCommandAliases(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-01 — verify p/c -> plane/create, p/m -> plane/mine")
}

func TestUnknownCommandSuggestion(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-01 — verify suggest-on-unknown with closest match")
}

func TestConnectCommand(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify email match and KV store persistence")
}

func TestConnectAlreadyConnected(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify message when already linked")
}

func TestObsidianSetup(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify dialog opens and config saves to KV")
}

func TestRequirePlaneConnection(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify guard blocks unconnected users")
}

func TestPlaneMine(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify assigned tasks list with emoji formatting")
}

func TestPlaneMineNoTasks(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify empty state message")
}

func TestPlaneStatus(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify project summary with state counts and progress bar")
}

func TestPlaneStatusProjectSelection(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify project name/identifier matching")
}
