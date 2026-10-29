# Scheduled DND

A Mattermost plugin for private, recurring Do Not Disturb schedules. Schedules
run on the server, persist in plugin KV storage, and work while users are offline.
No mobile changes, external database, cron service, or credentials are required.

**Release status:** Go race tests, frontend tests, TypeScript checking, and the
Linux amd64/arm64 plugin bundle build pass inside Docker. Live Mattermost and
HA acceptance remain unverified. See [verification](docs/verification.md) before
deployment.

## Compatibility

Target: Mattermost **11.7+**, including 11.7 ESR and 12.x, on Linux amd64/arm64.
The manifest enforces 11.7.0 as the minimum. This is a compatibility target, not
a tested server matrix. The SDK is `server/public v0.5.0`, matching the inspected
official starter. [Research and source references](docs/research.md) document
the supported SDK methods and their behavior.

Settings require a supported desktop/browser client. Native mobile clients do
not render plugin webapp UI; server-side DND applies without mobile changes.
The administrator must enable plugins and `ServiceSettings.EnableUserStatuses`.

## Build and test

Install only Docker with Docker Compose. Go 1.26.7, Node.js 24, npm, Python,
Make, and the C compiler needed by Go's race detector run inside the builder
container. Docker needs internet access to fetch images and dependencies.

```sh
# Linux/WSL: keep generated files owned by your current user.
export LOCAL_UID=$(id -u) LOCAL_GID=$(id -g)
docker compose build builder
docker compose run --rm builder
```

The default command installs dependencies, runs Go tests with the race detector,
frontend functional tests and TypeScript checking, builds both server targets,
and validates the archive. Frontend tests use Node's test runner with isolation
disabled so TypeScript test bodies execute in the current process.
The build creates:

```text
dist/org.mattermost.scheduled-dnd-<version>.tar.gz
```

The archive contains a top-level plugin ID directory, manifest, Linux amd64 and
arm64 executables, the webapp bundle, documentation, and dependency notices.
`make dist` only publishes the final archive name after all build steps succeed.
The repository is mounted at `/workspace`; generated files and `dist/` are
written directly into the checkout. Build caches and `webapp/node_modules` use
Docker volumes, so the host needs no language runtime or package manager.

Individual commands after dependency installation:

```sh
docker compose run --rm builder deps
docker compose run --rm builder test
docker compose run --rm builder dist
docker compose run --rm builder clean
```

Dependency versions and checksums are recorded in `go.mod`, `go.sum`, and
`webapp/package-lock.json`. Setup uses `npm ci` when the npm lockfile exists
and generates missing lockfiles on a new checkout. GitHub Actions runs the same
Docker Compose build and verification, then uploads the plugin archive and
dependency lockfiles as artifacts.
No Go/Node setup on the CI host is required.

Builds download the recorded Go dependency graph and compile with `-mod=readonly`.
When changing dependencies, run `docker compose run --rm --entrypoint go builder
mod tidy`, then review and commit the resulting `go.mod`/`go.sum` changes before
releasing.

If the host UID/GID changes, recreate the caches with `docker compose down
--volumes` before rebuilding. That removes only this Compose project's caches
and dependency volume; the source checkout and generated archives remain.

## Versioning and releases

Publish using **GitHub → Actions → Release new version → Run workflow**.
Leave the repository's default branch selected, choose `patch` (the default),
`minor`, or `major`, and click **Run workflow**. No local version edits or tags
are needed. The workflow file must first be present on the default branch.

The initial source version is `0.0.0`, a baseline rather than a published release.
No previous GitHub Release or Git tag is required:

| Choice | First release | Example after 1.2.3 |
| --- | --- | --- |
| patch (default) | 0.0.1 | 1.2.4 |
| minor | 0.1.0 | 1.3.0 |
| major | 1.0.0 | 2.0.0 |

The workflow reads the latest default-branch manifests, updates `plugin.json`,
`webapp/package.json`, and both npm lockfile version fields, then runs the Docker
tests and release build. Only after success does it commit the version change,
push the branch and `vX.Y.Z` tag atomically, and publish the installable archive
and SHA-256 checksum in GitHub Releases. The published version becomes the base
for the next run. Releases are serialized to avoid competing version increments.
Builds run directly in this workflow and do not rely on tag-triggered CI.

The repository needs GitHub Actions enabled. The publish job uses the built-in
`GITHUB_TOKEN` with `contents: write`; no personal token or extra secret is needed.
Repository/organization policy and branch protection must permit the bot's
version commit. The workflow does not force-push or bypass those protections.
If the branch advances during a build, both the version push and tag are rejected;
run a new release against the updated branch.

The release remains a draft until both assets upload. If publication fails after
the commit/tag push, those refs remain. Retry **failed jobs** to reuse the same
tested version; inspect/remove an incomplete draft first if one was created.
Do not rerun all jobs to recover that version, since the build job would calculate
the next increment. Existing tags for different source and published releases
are never overwritten. The first actual GitHub publication is a separate step
from preparing and testing this workflow locally.

For an unpublished local package, run `docker compose run --rm builder release`.
This builds the current manifest version and prepares its checksum and
`dist/release-notes.md`, without changing Git refs or contacting GitHub. The local
`version VERSION=X.Y.Z` command is available for development; normal releases
should use the manual GitHub workflow so versioning stays consistent.

## Install

1. Download the `.tar.gz` plugin asset from GitHub Releases, or build it above.
   GitHub's automatic **Source code** zip/tar.gz links are not plugin packages.
2. In **System Console → Plugins → Plugin Management**, upload the `.tar.gz`.
   Enable plugin uploads in server configuration if your installation requires it.
3. Enable **Scheduled DND**. Ensure user statuses are enabled.
4. Reload the Mattermost desktop/browser client. Open **main menu → Scheduled DND**.

Upgrades use the same upload procedure with a newer release; keep the plugin ID
unchanged to retain user schedules. No Marketplace listing or custom installer
is required for manual uploads. A public Marketplace listing and signed
distribution are not configured.

Manual web uploads require `PluginSettings.EnableUploads=true`. If your server
enforces `PluginSettings.RequirePluginSignature`, those uploads are disabled;
the administrator must arrange signed distribution with trusted keys instead.
The published SHA-256 file verifies integrity and is not a plugin signature.
See [Mattermost's plugin settings](https://docs.mattermost.com/administration-guide/configure/plugins-configuration-settings).

## Configure

Enable automatic DND, select an IANA timezone (for example `Europe/Warsaw`), add
one or more intervals, select their start weekdays, and save. Saturday and Sunday
can use separate intervals. Reset saves disabled defaults. See [UI documentation](docs/ui.md).

Example configuration (inside the API's `config` field):

```json
{
  "enabled": true,
  "timezone": "Europe/Warsaw",
  "schedule": [
    {"days": ["monday", "tuesday", "wednesday", "thursday", "friday"], "start": "17:00", "end": "08:00"}
  ]
}
```

Days always specify **the local day on which an interval starts**. Friday
17:00–08:00 therefore ends on Saturday. Start is inclusive; end is exclusive.
An end earlier than the start crosses midnight. Equal times are rejected.
There are at most 32 rules, each with distinct lowercase weekdays and HH:mm
times. Overlapping and touching intervals form one continuous union with the
latest end. A union covering the entire week is rejected because it has no
finite override boundary.

DST uses local calendar dates rather than 24-hour durations. In a spring clock
gap, missing times advance to the first valid wall minute; a collapsed interval
is skipped. In an autumn repeated hour, starts use the first occurrence and ends
use the last. Go's embedded timezone database is included; rebuild to update it.

## Manual controls and limits

- Existing DND is always preserved, including indefinite, longer, shorter,
  manual, or other-plugin DND. Manually selected Away/Offline and unknown status
  values are also preserved at activation. Automatic Away/Offline can enter DND.
- The plugin attempts activation at most once per continuous interval. If you
  change status after activation, it will not put you back into DND that interval.
- **Skip current interval** persists a temporary pause. To leave DND immediately,
  use Mattermost's status menu. The skip control never clears status.
- Edits after a current interval was handled take effect for future intervals;
  automation is deferred until both the old handled interval and the newly
  configured active interval end. Reset/disable/reenable retain that deferral.
  Existing timed DND retains its original expiration, including after a reset,
  plugin disable, or schedule edit.
- Mattermost expires timed DND itself, normally within its next minute job. The
  plugin never restores Online, Away, or Offline at expiration.
- The status API exposes no ownership token or conditional write. The plugin
  cannot eliminate a manual/third-party change racing the final status read and
  activation call. The displayed matching expiration is an observation, not
  proof of ownership. Integrations should avoid competing status automation.
- In the inspected server, another plugin's untimed `UpdateUserStatus(..., "dnd")`
  can retain a previous DND deadline. Native expiration may subsequently affect
  that status. Such integrations should use the timed setter with their chosen
  deadline (zero for indefinite); see the linked source in the research notes.
- A crash or API error after reserving an activation forfeits that interval.
  Unconfirmed calls are intentionally not retried; the panel exposes the state.
  Unprocessed active intervals recover on the next scheduler scan after restart.

## Architecture and HA

The backend embeds the official `plugin.MattermostPlugin`; React is supplied by
the Mattermost host. The settings panel uses supported main-menu/root-component
registration and theme variables instead of private core component imports.

A cluster job is scheduled on 30-second rounded boundaries. It pages through
plugin KV keys and evaluates each saved configuration without needing a client
connection. Each user has one atomic record containing configuration, a revision,
and the last interval decision. Disabled schedules remain in KV, so resetting
cannot erase a safety decision or race key deletion.

`cluster.Schedule` coordinates background execution, while a per-user cluster
mutex serializes scheduler and configuration operations. A KV compare-and-set
reserves each activation before calling `SetUserStatusTimedDND` with the interval
end in **Unix seconds**. Durable decisions survive process restarts and changes
of job executor. No in-memory leader state is required. CAS also rejects stale
saves from another tab or node.

Cluster leases are not fencing tokens for the status API. Arbitrarily long
process pauses, partitions, and external status writers cannot be made atomic
with plugin KV. Key enumeration is eventually consistent when other operations
create/remove transient mutex keys; a concurrent new configuration can be seen
on the following scan. Scan time increases with configured users and API latency.
The job logs a warning if a pass exceeds one minute; operators must verify their
deployment's scan latency. The one-minute target is not a hard real-time SLA.

## API and security

All paths are under `/plugins/org.mattermost.scheduled-dnd` (and any SiteURL subpath).

| Method | Path | Request |
| --- | --- | --- |
| GET | `/api/v1/config` | Read own config and live automation view |
| PUT | `/api/v1/config` | JSON `{ "config": {...}, "revision": 0 }` |
| DELETE | `/api/v1/config` | `If-Match: "<revision>"`; save disabled defaults |
| POST | `/api/v1/skip` | No body; skip current enabled interval |

Mattermost supplies the authenticated `Mattermost-User-ID`; user IDs are never
accepted from request bodies, paths, or query parameters. The backend verifies
an active non-bot user. Mutations require `X-Requested-With: XMLHttpRequest` and
reject cross-site browser requests. No CORS access is enabled. JSON size is
bounded, unknown fields and trailing values are rejected, and the server validates
configuration independently of the UI. Responses disable caching. No tokens or
settings are put in browser storage. Deleted/deactivated users are skipped by
the scheduler; their private KV records remain until plugin data is removed.

Revision conflicts return 409. Other responses include 400 for malformed input,
401/403 for authentication/access failures, 415 for non-JSON saves, and 428 for
reset without a revision. Storage/API errors are logged server-side and returned
as generic failures. Plugin KV is private through this API, not encryption against
Mattermost administrators or database operators.

Slash commands are not implemented; configuration and overrides use the panel.

## Troubleshooting

- **Menu missing:** enable the plugin and reload a supported desktop/browser
  client. The native mobile app does not display the configuration panel.
- **Plugin will not activate:** check that user statuses are enabled and the
  server meets the manifest's minimum version.
- **Schedule did not activate:** check enabled state, start days, timezone, server
  clock, existing manual status, and the panel's current automation state. A
  preserved, skipped, failed, or pending interval will not be retried.
- **DND remains after editing/reset:** the original timed DND expires naturally.
  Use the Mattermost status menu to leave it immediately.
- **Save conflict:** use Reload saved configuration, then apply your edits again.
- **Delayed activation/expiry:** allow the scheduler and native expiry jobs to
  run; inspect server logs for `Scheduled DND` failures or a scan exceeding one
  minute. Keep cluster clocks synchronized and investigate KV/API latency.
- **Build failure:** check Docker's image/dependency network access.
  Run `docker compose build builder` and `docker compose run --rm builder`.
  A Docker socket permission error concerns host access to the daemon, not a
  missing Go installation. First verify `docker compose ps` works in your shell.

## License

Plugin code is MIT. Upstream dependencies retain their own licenses. The
starter and SDK references are acknowledged in `NOTICE`; release builds collect
linked Go dependency notices and include the Apache-2.0 license text.
