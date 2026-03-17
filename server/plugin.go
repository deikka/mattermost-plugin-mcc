package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// Plugin implements the Mattermost plugin interface.
type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *configuration

	client      *pluginapi.Client
	botUserID   string
	router      *mux.Router
	planeClient *plane.Client
	store       *store.Store
	digestJob   *cluster.Job
}

// OnActivate is invoked when the plugin is activated. If an error is returned, the plugin
// will be deactivated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	// Load configuration first (Pitfall 5: config not available in OnActivate)
	if err := p.OnConfigurationChange(); err != nil {
		return errors.Wrap(err, "failed to load initial configuration")
	}

	// Create bot account
	if err := p.ensureBot(); err != nil {
		return err
	}

	// Initialize KV store
	p.store = store.New(p.API)

	// Initialize Plane client
	cfg := p.getConfiguration()
	p.planeClient = plane.NewClient(cfg.PlaneURL, cfg.PlaneAPIKey, cfg.PlaneWorkspace)

	// Register slash commands
	if err := p.registerCommands(); err != nil {
		return errors.Wrap(err, "failed to register commands")
	}

	// Initialize HTTP router
	p.initRouter()

	// Start digest scheduler (non-critical -- don't fail activation)
	if err := p.startDigestScheduler(); err != nil {
		p.API.LogError("Failed to start digest scheduler", "error", err.Error())
	}

	// Non-blocking health check (don't block activation on external service)
	go p.validatePlaneConnection()

	return nil
}

// OnDeactivate is invoked when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	p.stopDigestScheduler()
	return nil
}

// initRouter initializes the gorilla/mux router and registers all API routes.
// Extracted from OnActivate so tests can call it independently.
func (p *Plugin) initRouter() {
	p.router = mux.NewRouter()
	p.initAPI()
}

// ServeHTTP delegates HTTP requests to the plugin router.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

// validatePlaneConnection performs a non-blocking health check against the configured
// Plane instance. If the connection fails, it notifies the system admin via DM from
// the bot account. This never blocks plugin activation.
func (p *Plugin) validatePlaneConnection() {
	cfg := p.getConfiguration()

	if strings.TrimSpace(cfg.PlaneURL) == "" {
		p.API.LogInfo("Plane URL not configured, skipping connection validation")
		return
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s/projects/",
		strings.TrimRight(cfg.PlaneURL, "/"),
		cfg.PlaneWorkspace,
	)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		p.API.LogError("Failed to create Plane health check request", "error", err.Error())
		return
	}
	req.Header.Set("X-API-Key", cfg.PlaneAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		p.notifyAdminPlaneError(cfg.PlaneURL)
		p.API.LogWarn("Plane connection check failed", "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.notifyAdminPlaneError(cfg.PlaneURL)
		p.API.LogWarn("Plane connection check returned non-200",
			"status", resp.StatusCode,
			"url", url,
		)
		return
	}

	p.API.LogInfo("Plane connection validated successfully", "url", cfg.PlaneURL)
}

// notifyAdminPlaneError sends a DM to system admins informing them of a Plane connection issue.
func (p *Plugin) notifyAdminPlaneError(planeURL string) {
	message := fmt.Sprintf(
		"Could not connect to Plane at **%s**. "+
			"Please verify your configuration in **System Console > Plugins > Mattermost Command Center**.",
		planeURL,
	)

	// Get system admins to notify
	admins, appErr := p.API.GetUsers(&model.UserGetOptions{
		Role:    "system_admin",
		Page:    0,
		PerPage: 100,
	})
	if appErr != nil {
		p.API.LogError("Failed to list admins for Plane connection notification", "error", appErr.Error())
		return
	}

	for _, admin := range admins {
		channel, appErr := p.API.GetDirectChannel(p.botUserID, admin.Id)
		if appErr != nil {
			p.API.LogError("Failed to get DM channel with admin", "admin", admin.Id, "error", appErr.Error())
			continue
		}

		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: channel.Id,
			Message:   message,
		}
		if _, appErr := p.API.CreatePost(post); appErr != nil {
			p.API.LogError("Failed to notify admin about Plane connection", "admin", admin.Id, "error", appErr.Error())
		}
	}
}

// MessageHasBeenPosted is invoked after a message has been posted.
// Used for link unfurling of Plane work item URLs.
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	p.handleLinkUnfurl(post)
}

// markPluginAction records that the plugin originated an action on a work item.
// This prevents self-notification when Plane fires a webhook for the same action.
// The entry expires after 5 minutes (300 seconds).
func (p *Plugin) markPluginAction(workItemID string) {
	_, _ = p.API.KVSetWithOptions("plugin_action_"+workItemID, []byte("1"), model.PluginKVSetOptions{ExpireInSeconds: 300})
}

func main() {
	plugin.ClientMain(&Plugin{})
}
