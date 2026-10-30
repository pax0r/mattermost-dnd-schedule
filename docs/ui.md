# User interface

Open **Settings → Scheduled DND** in the desktop or browser client. The panel
uses the supported `registerUserSettings` custom-section API and Mattermost's
host React, theme variables, and button conventions. Configuration stays in
plugin KV; the custom section manages its own save/reset controls.

```text
Settings → Scheduled DND
Protect your quiet hours. Your schedule runs on the server…

┌ Current automation status ──────────────────────┐
│ DND is active with the scheduled expiration.    │
│ Mattermost status: dnd until …                  │
│ Current interval ends …                        │
│ Next interval starts …                         │
└────────────────────────────────────────────────┘

[✓] Enable automatic DND
Timezone: [ Europe/Warsaw                      ▾ ]

Interval 1
[✓] Mon [✓] Tue [✓] Wed [✓] Thu [✓] Fri [ ] Sat [ ] Sun
Start [17:00]   End [08:00]             [Remove]
Ends the following morning.

[+ Add interval]

Schedule summary
• mon, tue, wed, thu, fri: 17:00–08:00 next day (Europe/Warsaw)

[Save schedule] [Reset configuration]
─────────────────────────────────────────────────
[Skip current interval]
To end active DND, use your Mattermost status menu.
```

The editor supports multiple intervals, independent weekend selections, and
searchable timezone suggestions through an editable datalist. Save validates
each interval. The server remains authoritative for IANA timezone validation.
The summary describes the draft; automation status describes the saved schedule.
Status refresh does not overwrite unsaved edits. A stale revision produces a
conflict with a reload control; reloading discards the current draft.

**Reset configuration** confirms before saving disabled defaults and clearing
the interval list. It retains a current interval's override/decision for safety.
**Skip current interval** immediately persists a temporary automation pause; it
does not change the user's current Mattermost status. Changes in other tabs are
detected through configuration revisions.

Native mobile clients do not load this settings panel. A schedule saved from
desktop/web still runs while the user is offline and applies to their server
presence without modifying the mobile app.

## Live acceptance walkthrough

Run this against each target server release before publishing a release:

1. Install and enable the bundle. Open the panel, save an interval starting a
   few minutes ahead, close the client, and verify DND starts server-side and
   expires through Mattermost's timed-DND job.
2. Reopen the panel and check saved values, theme colors, keyboard navigation,
   native settings navigation, validation messages, and current/next intervals.
3. Create another user's schedule. Verify the two configurations are independent.
4. In two tabs, load the same revision and save different edits. The second
   save must show a conflict; reload must show the accepted configuration.
5. During an active interval, select Online, Away, or Offline. Wait through
   multiple scheduler runs and the original expiration. The plugin must not
   reapply or restore status. Test longer manual DND before activation too.
6. Skip an interval before its first evaluation; restart the plugin and verify
   no activation. Edit, reset, disable, and reenable during an active interval;
   check the documented deferral and original timed-DND expiration.
7. On two cluster nodes, route saves through either node, restart the node
   executing jobs, and check that a handled interval is not activated twice.

This walkthrough is a release requirement; it has not been executed in this
workspace. The text mockup is UI documentation, not a live-server screenshot.
