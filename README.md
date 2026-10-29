# Scheduled DND

A Mattermost plugin for recurring Do Not Disturb schedules. Each user's schedule
runs server-side, persists across restarts, and works while they are offline.

## Requirements

- Mattermost 11.7+ on Linux amd64 or arm64, with plugins and user statuses enabled.
- A desktop/browser client to configure schedules. Native mobile clients use the
  resulting server-side DND status but do not display the settings panel.
- Docker with Compose to build and test; no local Go, Node, or npm is required.

Live Mattermost and HA compatibility have not yet been validated.

## Installation

1. Download the plugin `.tar.gz` asset from [GitHub Releases](https://github.com/pax0r/mattermost-dnd-schedule/releases),
   or build it using the instructions below. GitHub's **Source code** archives
   are not installable plugins.
2. As a system administrator, upload it in **System Console → Plugins → Plugin Management**.
3. Enable **Scheduled DND**, reload the client, and open **main menu → Scheduled DND**.

Manual uploads require `PluginSettings.EnableUploads=true`. Deployments enforcing
`RequirePluginSignature` disable web uploads and need signed distribution with
trusted keys. The SHA-256 checksum supplied with a release is not a signature.
See [Mattermost's plugin settings](https://docs.mattermost.com/administration-guide/configure/plugins-configuration-settings).

To upgrade, upload the newer plugin bundle. Keep the plugin ID unchanged to
retain user configuration.

## Configuration

Enable automation, choose an IANA timezone such as `Europe/Warsaw`, add intervals,
select their start weekdays, and save. Weekend days can use separate intervals.
The panel shows the schedule summary and current automation status.
[UI documentation](docs/ui.md) includes an example layout.

Example configuration:

```json
{
  "enabled": true,
  "timezone": "Europe/Warsaw",
  "schedule": [
    {
      "days": ["monday", "tuesday", "wednesday", "thursday", "friday"],
      "start": "17:00",
      "end": "08:00"
    }
  ]
}
```

Days specify when an interval **starts**: Friday 17:00–08:00 ends on Saturday.
Start is inclusive and end is exclusive. Overlapping or touching intervals merge
into one continuous interval. Equal start/end times and continuous 24/7 coverage
are rejected; a configuration can contain up to 32 intervals.

During a spring DST gap, missing times advance to the first valid wall minute;
a collapsed interval is skipped. During an autumn repeated hour, starts use the
first occurrence and ends use the last.

## Manual controls and limitations

- Existing DND is preserved regardless of its source or duration. Manually
  selected Away/Offline statuses are also preserved at activation.
- Automation attempts activation once per continuous interval. A later manual
  status change prevents reapplication for the rest of that interval.
- **Skip current interval** pauses automation. To leave already-active DND,
  use Mattermost's status menu.
- Editing a handled interval defers automation until both the old interval and
  the newly configured active interval end. Resetting or disabling automation
  leaves existing timed DND to expire at its original deadline.
- Mattermost handles expiration. The plugin never explicitly clears or restores
  status, because the API provides no reliable ownership marker or conditional
  status write. Manual or third-party changes can race activation.
- A failed or unconfirmed activation is not retried during that interval.
  Unprocessed active intervals are evaluated after restart.

[SDK integration notes](docs/research.md) explain status ownership, expiration,
interaction with other plugins, and cluster limitations.

## Architecture and HA

The Go backend stores per-user configuration and interval decisions in plugin KV.
The React/TypeScript panel uses Mattermost's host React and theme variables.
No external database, cron service, or mobile changes are required.

A cluster job evaluates schedules on 30-second boundaries. Per-user cluster
mutexes and KV compare-and-set coordinate activation and configuration changes.
Durable decisions survive restarts and executor changes. Timed DND uses the
interval's end in Unix seconds.

Cluster leases cannot fence the status API, so arbitrary pauses, partitions,
and competing integrations cannot be made transactional. Scan latency grows
with configured users and API latency; a pass exceeding one minute is logged.

## Build and test

Docker needs network access to fetch images and dependencies.

```sh
# Linux/WSL: keep generated files owned by your user.
export LOCAL_UID=$(id -u) LOCAL_GID=$(id -g)
docker compose build builder
docker compose run --rm builder
```

This runs Go race tests, frontend and release-tool tests, TypeScript checking,
and builds and validates both server architectures. The plugin bundle is written
to `dist/org.mattermost.scheduled-dnd-<version>.tar.gz`.

For individual steps, use `docker compose run --rm builder` followed by `deps`,
`test`, `dist`, or `clean`. Caches and frontend dependencies use Docker volumes.
Builds use pinned dependencies and readonly Go module resolution. To change Go
dependencies, run:

```sh
docker compose run --rm --entrypoint go builder mod tidy
```

Review and commit the resulting lockfile changes before releasing.
[Verification details](docs/verification.md) distinguish automated coverage from
pending live-server checks.

## Releases

Open **GitHub → Actions → Release new version → Run workflow**, keep the default
branch selected, and choose `patch` (default), `minor`, or `major`.
From the initial `0.0.0` baseline, these produce `0.0.1`, `0.1.0`, or `1.0.0`.

After tests and the Docker build pass, the workflow commits the version change,
pushes the branch and tag atomically, and publishes the bundle and SHA-256 file
in GitHub Releases. It uses `GITHUB_TOKEN`; repository permissions and branch
protection must permit the bot's version commit.

If the branch advances during a build, start a new run. If publication fails
after the tag was pushed, retry **failed jobs** to reuse that version; remove
an incomplete release draft first if necessary. Rerunning all jobs calculates
the next version.

For an unpublished local bundle, run `docker compose run --rm builder release`.

## Troubleshooting

- **Menu missing:** enable the plugin and reload a desktop/browser client.
- **No activation:** check timezone, start days, enabled state, manual status,
  and the panel's automation status. Skipped, preserved, failed, or pending
  intervals are not retried.
- **Save conflict:** reload the saved configuration and reapply your edits.
- **Delayed activation/expiration:** check server logs for `Scheduled DND`, KV/API
  latency, and synchronized cluster clocks. Allow for the scheduler and native
  expiry jobs to run.
- **Build failure:** check Docker daemon access and image/dependency network
  access. If changing the build UID/GID, remove the old cache volumes with
  `docker compose down --volumes` before rebuilding.

## License

[MIT](LICENSE). Dependencies retain their own licenses; see [NOTICE](NOTICE).
Release bundles include linked Go dependency license notices.
