# Mattermost Command Center

A Mattermost plugin that turns chat into a task management hub. Create, query, and track [Plane](https://plane.so) tasks without leaving Mattermost.

Built for small teams (2-10 people) where Mattermost is the communication hub.

## Features

**Task Creation**

- `/task plane create` — interactive dialog with project, priority, assignee, labels
- `/task p c Fix the login bug` — quick inline creation
- Right-click any message → "Create Task in Plane" with text pre-populated

**Queries**

- `/task plane mine` — your assigned tasks across projects
- `/task plane status [project]` — project summary with progress bar

**Channel-Project Binding**

- `/task plane link [project]` — bind a channel to a Plane project
- All commands in a bound channel auto-use the linked project
- `/task plane unlink` — remove binding

**Notifications**

- Plane webhooks → channel notifications for state, assignee, priority, and comment changes
- `/task plane notifications on|off` — toggle per channel
- `/task plane digest daily|weekly|off [hour]` — periodic project summaries

**Link Unfurling**

Paste a Plane task URL and the bot replies with a preview card showing title, status, priority, and assignee.

**Other**

- `/task connect` — link your Mattermost account to Plane via email match
- `/task help` — command reference
- Levenshtein-based suggestions for typos

## Requirements

- Mattermost Server 9.0+
- Plane (self-hosted) with API access
- Go 1.21+ (to build)
- Node.js 18+ (to build webapp)

## Installation

Build the plugin bundle:

```sh
make bundle
```

Upload `com.klab.mattermost-command-center.tar.gz` via **System Console → Plugin Management → Upload Plugin**.

Or deploy directly:

```sh
export MM_SERVICESETTINGS_SITEURL=https://mattermost.example.com
export MM_ADMIN_TOKEN=your-admin-token
make deploy
```

## Configuration

In **System Console → Plugins → Mattermost Command Center**:

| Setting | Description |
|---------|-------------|
| Plane URL | Base URL of your Plane instance |
| Plane API Key | API token from Plane Settings → API Tokens |
| Plane Workspace Slug | Workspace slug from your Plane URL |
| Plane Webhook Secret | HMAC secret for verifying incoming webhooks |

### Webhooks Setup

To receive notifications from Plane:

1. Go to Plane → Workspace Settings → Webhooks
2. Add webhook URL: `https://your-mattermost.com/plugins/com.klab.mattermost-command-center/api/v1/webhook/plane`
3. Set the same secret in both Plane and the plugin settings
4. Select events: Issue updates, Comment creates

## Development

```sh
# Run tests
make test

# Build server binaries
make build

# Build webapp
make webapp

# Lint
make check-style
```

## License

MIT
