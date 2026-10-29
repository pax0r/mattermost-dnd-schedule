# Verified Mattermost integration

Research date: 2026-10-29. These are API compatibility findings, not a live-server certification.

## Foundation and compatibility

The [official starter](https://github.com/mattermost/mattermost-plugin-starter-template)
provides the layout used here: `plugin.json`, `server/main.go`, a webapp entry point,
cluster jobs, and an archive containing server executables and `webapp/dist/main.js`.
Its [current module](https://github.com/mattermost/mattermost-plugin-starter-template/blob/master/go.mod)
uses `server/public v0.5.0` and Go 1.26.7; this plugin pins those versions.
The build retains the starter's host-provided React convention with a smaller esbuild pipeline.

The plugin targets Mattermost 11.7+ (11.7 ESR and 12.x). Older releases are not a
support target even though individual APIs predate them. Mattermost's
[upgrade notes](https://docs.mattermost.com/upgrade/important-upgrade-notes)
identify 11.7 as the successor to the expired 10.11 ESR. No live version matrix
has been exercised in this workspace.

## Status APIs

The [official plugin API](https://github.com/mattermost/mattermost/blob/master/server/public/plugin/api.go)
declares:

```go
GetUserStatus(userID string) (*model.Status, *model.AppError)
UpdateUserStatus(userID, status string) (*model.Status, *model.AppError)
SetUserStatusTimedDND(userId string, endtime int64) (*model.Status, *model.AppError)
```

The first two require server 5.2; timed DND requires 5.35. `UpdateUserStatus`
selects online/away/offline/dnd and offers no conditional update or ownership token.
This plugin never uses it to restore a status.

The [status model](https://github.com/mattermost/mattermost/blob/master/server/public/model/status.go)
explicitly defines `DNDEndTime` in **Unix seconds**, unlike most Mattermost timestamps.
`Manual` is also set by plugins; it cannot identify the actor. `PrevStatus` is not
serialized in JSON. No source-plugin ID or status revision is exposed.

[Platform status handling](https://github.com/mattermost/mattermost/blob/master/server/channels/app/platform/status.go)
sets timed DND as manual, records the previous status, and rounds expiry down to
the minute. The [expiry query](https://github.com/mattermost/mattermost/blob/master/server/channels/store/sqlstore/status_store.go)
only updates rows still in DND with a positive expired deadline. It restores
`PrevStatus` and resets the deadline/manual flag. A separately selected non-DND
status is therefore outside that query. Expiration runs once per minute.

The [REST status handler](https://github.com/mattermost/mattermost/blob/master/server/channels/api4/status.go)
passes a user's chosen DND deadline, including zero for indefinite DND, to the
timed setter. The [plugin API implementation](https://github.com/mattermost/mattermost/blob/master/server/channels/app/plugin_api.go)
instead routes `UpdateUserStatus(userID, "dnd")` through the untimed setter,
which in the inspected source does not clear an existing deadline. A different
plugin using that path may inherit a scheduled deadline and later be affected
by native expiration. This plugin cannot repair that safely without ownership
metadata. The timed setter also reads status after a void setter, so its return
is confirmation of observed state, not a transactional persistence guarantee.

## Persistence and HA

Plugin-scoped APIs include `KVGet(key) ([]byte, *model.AppError)`,
`KVList(page, perPage) ([]string, *model.AppError)`, and
`KVCompareAndSet(key, oldValue, newValue) (bool, *model.AppError)`.
Missing `KVGet` returns nil; CAS with nil old value creates an absent key. CAS
returns false without error on contention. This permits a single atomic record
for each user's configuration and interval decision, without an auxiliary index.

The starter schedules a cluster job using `cluster.Schedule(api, key,
cluster.MakeWaitForRoundedInterval(interval), callback) (*cluster.Job, error)`;
`Close() error` stops local execution. The [cluster package](https://pkg.go.dev/github.com/mattermost/mattermost/server/public/pluginapi/cluster)
coordinates callbacks across instances using KV. It also provides
`NewMutex(api, key) (*Mutex, error)`, `LockWithContext(context.Context) error`,
and `Unlock()`. Mutexes have renewable leases; they are not status API fencing.
This implementation combines those primitives with a persistent CAS decision
before any status write. A crash between the decision and API call intentionally
forfeits that interval instead of risking a duplicate activation.

## UI and calendar reference

The [webapp reference](https://docs.mattermost.com/developers/integrate/reference/webapp)
documents `registerUserSettings`, `registerRootComponent`, and
`registerMainMenuAction`. The settings schema persists values in Mattermost
preferences. A main-menu panel is chosen for the dynamic interval editor and
KV-backed configuration, using the host's React and theme CSS variables. Internal
webapp components are not a stable plugin import API.

The [Google Calendar entry point](https://github.com/mattermost/mattermost-plugin-google-calendar/blob/master/server/main.go)
delegates to the shared Microsoft Calendar engine. Its
[availability implementation](https://github.com/mattermost/mattermost-plugin-mscalendar/blob/master/calendar/engine/availability.go)
polls users and reads current status before syncing calendar availability.
That integration is useful precedent for server-side synchronization; it does
not add reliable status ownership metadata to the SDK.

## Safety boundary

There is no atomic compare-and-set for user status. A manual or third-party
change between the final read and timed-DND call can race activation. A process
pause beyond a cluster mutex lease also cannot be fenced at the status API.
Do not claim perfect ownership detection or distributed exactly-once status
transactions. Preserve every existing DND, make at most one activation attempt
per continuous interval, and never explicitly clear status. The skip control
prevents activation; users clear already-active DND with Mattermost's status menu.
