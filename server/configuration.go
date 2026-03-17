package main

// configuration captures the plugin's external configuration as exposed in the Mattermost
// System Console. Changes are persisted to the server's config store.
type configuration struct {
	PlaneURL       string `json:"PlaneURL"`
	PlaneAPIKey    string `json:"PlaneAPIKey"`
	PlaneWorkspace string `json:"PlaneWorkspace"`
}
