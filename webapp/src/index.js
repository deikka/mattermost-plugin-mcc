// Mattermost Command Center - Webapp Plugin
// Registers the "Create Task in Plane" post dropdown menu action.
//
// Dialog opening uses store.dispatch to bypass the trigger_id requirement.
// registerPostDropdownMenuAction does NOT provide a trigger_id, and the
// /api/v4/actions/dialogs/open REST endpoint requires one. By dispatching
// the openInteractiveDialog action directly on the Redux store, we open
// the dialog modal client-side without trigger_id validation.

class MccPlugin {
    initialize(registry, store) {
        this.registry = registry;
        this.store = store;

        registry.registerPostDropdownMenuAction(
            'Create Task in Plane',
            this.handleCreateTask.bind(this),
        );
    }

    async handleCreateTask(postId) {
        try {
            const response = await fetch(
                '/plugins/com.klab.mattermost-command-center/api/v1/action/create-task-from-message',
                {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    credentials: 'include',
                    body: JSON.stringify({ post_id: postId }),
                },
            );

            if (!response.ok) {
                const err = await response.json();
                console.error('MCC: Failed to create task from message:', err.error || response.statusText);
                return;
            }

            const dialogConfig = await response.json();

            // Open the dialog client-side via Redux store dispatch.
            // This bypasses trigger_id validation since we're opening
            // the dialog directly in the webapp UI, not via REST API.
            this.store.dispatch({
                type: 'RECEIVED_DIALOG',
                data: {
                    url: dialogConfig.url,
                    dialog: dialogConfig.dialog,
                },
            });
        } catch (err) {
            console.error('MCC: Error in create task from message:', err);
        }
    }

    uninitialize() {
        // Cleanup if needed
    }
}

window.registerPlugin('com.klab.mattermost-command-center', new MccPlugin());
