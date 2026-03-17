package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// startDigestScheduler initializes the periodic digest check loop using cluster.Schedule
// for HA-safe single execution across plugin instances.
func (p *Plugin) startDigestScheduler() error {
	job, err := cluster.Schedule(
		p.API,
		"PlaneDigestScheduler",
		cluster.MakeWaitForRoundedInterval(1*time.Minute),
		p.runDigestCheck,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule digest job: %w", err)
	}
	p.digestJob = job
	return nil
}

// stopDigestScheduler stops the periodic digest check loop.
func (p *Plugin) stopDigestScheduler() {
	if p.digestJob != nil {
		if err := p.digestJob.Close(); err != nil {
			p.API.LogError("Failed to close digest job", "error", err.Error())
		}
	}
}

// runDigestCheck iterates over channels with active digest configs and
// sends digest posts to those whose schedule is due.
func (p *Plugin) runDigestCheck() {
	// List all KV keys to find digest configs
	keys, appErr := p.API.KVList(0, 100)
	if appErr != nil {
		p.API.LogError("Failed to list KV keys for digest check", "error", appErr.Error())
		return
	}

	now := time.Now()

	for _, key := range keys {
		if !strings.HasPrefix(key, "digest_config_") {
			continue
		}

		channelID := strings.TrimPrefix(key, "digest_config_")

		// Read digest config
		config, err := p.store.GetDigestConfig(channelID)
		if err != nil {
			p.API.LogError("Failed to read digest config", "channel", channelID, "error", err.Error())
			continue
		}
		if config == nil || config.Frequency == "off" {
			continue
		}

		// Check if digest is due based on frequency and hour
		if !p.isDigestDue(config, now) {
			continue
		}

		// Check last digest timestamp to prevent re-posting
		if p.isDigestAlreadyPosted(channelID, config, now) {
			continue
		}

		// Get channel binding to find the project
		binding, err := p.store.GetChannelBinding(channelID)
		if err != nil {
			p.API.LogError("Failed to get channel binding for digest", "channel", channelID, "error", err.Error())
			continue
		}
		if binding == nil {
			continue
		}

		// Fetch work items and build digest
		workItems, err := p.planeClient.ListProjectWorkItems(binding.ProjectID)
		if err != nil {
			p.API.LogError("Failed to list work items for digest", "channel", channelID, "error", err.Error())
			continue
		}

		digestContent := p.buildDigestPost(binding, workItems)

		// Post the digest as a visible channel post (not ephemeral)
		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: channelID,
			Message:   digestContent,
		}
		if _, appErr := p.API.CreatePost(post); appErr != nil {
			p.API.LogError("Failed to post digest", "channel", channelID, "error", appErr.Error())
			continue
		}

		// Update last digest timestamp
		timestampStr := []byte(strconv.FormatInt(now.Unix(), 10))
		if _, appErr := p.API.KVSetWithOptions("digest_last_"+channelID, timestampStr, model.PluginKVSetOptions{}); appErr != nil {
			p.API.LogError("Failed to update digest timestamp", "channel", channelID, "error", appErr.Error())
		}
	}
}

// isDigestDue checks if the current time matches the configured digest schedule.
func (p *Plugin) isDigestDue(config *store.DigestConfig, now time.Time) bool {
	switch config.Frequency {
	case "daily":
		return now.Hour() == config.Hour
	case "weekly":
		return now.Hour() == config.Hour && now.Weekday() == time.Weekday(config.Weekday)
	default:
		return false
	}
}

// isDigestAlreadyPosted checks whether a digest has already been posted in the current period.
// For daily digests: checks if last post was within the same calendar hour.
// For weekly digests: checks if last post was within the same calendar day.
func (p *Plugin) isDigestAlreadyPosted(channelID string, config *store.DigestConfig, now time.Time) bool {
	data, appErr := p.API.KVGet("digest_last_" + channelID)
	if appErr != nil || data == nil {
		return false
	}

	lastTimestamp, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return false
	}

	lastTime := time.Unix(lastTimestamp, 0)

	switch config.Frequency {
	case "daily":
		// Same calendar hour = already posted
		return lastTime.Year() == now.Year() &&
			lastTime.YearDay() == now.YearDay() &&
			lastTime.Hour() == now.Hour()
	case "weekly":
		// Same calendar day = already posted
		return lastTime.Year() == now.Year() &&
			lastTime.YearDay() == now.YearDay()
	default:
		return false
	}
}

// buildDigestPost builds the markdown content for a periodic project digest.
// The digest shows state counters, progress bar, and a link to the project in Plane.
func (p *Plugin) buildDigestPost(binding *store.ChannelProjectBinding, workItems []plane.WorkItem) string {
	// Group by state group
	groupCounts := map[string]int{
		"backlog":   0,
		"unstarted": 0,
		"started":   0,
		"completed": 0,
		"cancelled": 0,
	}
	for _, item := range workItems {
		group := item.StateGroup
		if group == "" {
			group = "backlog"
		}
		groupCounts[group]++
	}

	open := groupCounts["backlog"] + groupCounts["unstarted"]
	inProgress := groupCounts["started"]
	done := groupCounts["completed"]
	total := len(workItems)

	// Calculate progress percentage
	percent := 0
	if total > 0 {
		percent = (done * 100) / total
	}

	bar := progressBar(done, total, 20)

	// Build project URL
	cfg := p.getConfiguration()
	planeBaseURL := strings.TrimRight(cfg.PlaneURL, "/")
	workspace := cfg.PlaneWorkspace
	projectURL := fmt.Sprintf("%s/%s/projects/%s/work-items/", planeBaseURL, workspace, binding.ProjectID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Resumen del Proyecto: %s**\n\n", binding.ProjectName))
	sb.WriteString("| Estado | Cantidad |\n")
	sb.WriteString("|--------|----------|\n")
	sb.WriteString(fmt.Sprintf("| :white_circle: Abierto | %d |\n", open))
	sb.WriteString(fmt.Sprintf("| :large_blue_circle: En Progreso | %d |\n", inProgress))
	sb.WriteString(fmt.Sprintf("| :white_check_mark: Hecho | %d |\n\n", done))
	sb.WriteString(fmt.Sprintf("**Progreso:** %s %d%%\n", bar, percent))
	sb.WriteString(fmt.Sprintf("**Total:** %d tareas\n\n", total))
	sb.WriteString(fmt.Sprintf("[Abrir en Plane](%s)\n\n", projectURL))
	sb.WriteString("_Resumen automatico -- Configura con `/task plane digest off` para desactivar_")

	return sb.String()
}
