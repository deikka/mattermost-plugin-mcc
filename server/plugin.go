package main

import (
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// Plugin implements the Mattermost plugin interface.
type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *configuration

	// client will be initialized in OnActivate (Plan 01-01).
	botUserID string
	router    *mux.Router
}

func main() {
	plugin.ClientMain(&Plugin{})
}
