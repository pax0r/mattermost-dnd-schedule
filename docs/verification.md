# Verification status

Verified on 2026-10-29 using the repository's Docker Compose builder. No host Go,
Node, npm, Python, or Make installation is needed for this workflow.

## Executed successfully

- `docker compose config --quiet`: Compose configuration accepted.
- `docker compose build builder`: toolchain image built successfully with Go
  1.26.7, Node 24, npm, Python, Make, gcc, and Git.
- `docker compose run --rm builder`: dependency resolution, tests, compilation,
  packaging, and archive validation all completed successfully.
- Go tests passed with `go test -race -count=1 ./server/...`, covering schedule
  boundaries, overnight/weekend intervals, timezone differences, Warsaw DST
  gaps/folds, overlaps, disabled/malformed configuration, restart recovery,
  manual overrides, concurrent CAS decisions, executor changes, schedule edits,
  missing/deactivated users, API authentication/isolation, and malformed requests.
- All 10 frontend functional tests passed, covering configuration validation,
  overnight summaries, weekly unions, request identity isolation, revisions,
  and API failures. Full React/JSX TypeScript checking passed.
- Twelve Python release tests passed, covering synchronized version updates,
  first-release and later major/minor/patch increments, validation, checksums,
  notes, and real Git operations against temporary local bare repositories.
  Git tests verify the first commit/tag push, reuse of an exact remote release
  on retry, atomic rejection when the branch advances, and tag collision handling.
- Six publisher tests with an injected GitHub client cover complete uploads,
  failed uploads remaining unpublished, checksum/tag/commit rejection before
  creating a release, and prerelease marking. They make no actual GitHub requests.
- The frontend bundle and both Linux amd64/arm64 server executables built.
  An isolated container copy starting at `0.0.0` successfully bumped to `0.0.1`,
  ran all tests and type checks, built and validated the installable archive,
  and generated its SHA-256 file and notes. The source checkout remains at the
  unreleased `0.0.0` baseline; no existing release or tag was needed.
- Manual workflow YAML and configuration were checked: `workflow_dispatch`,
  major/minor/patch choices, patch default, serialized releases, and write
  permissions restricted to the publish job.
- Dependency checksums/lockfiles and linked Go dependency license notices were
  generated. Build output belongs to the configured host UID/GID.
- A separate Node VM check confirmed browser bundle registration of the matching
  manifest ID, root component, menu action, and cleanup event with a stub host.

## Not yet verified

- The GitHub Actions workflow uses the same Compose commands, but has not been
  executed on GitHub from this workspace.
- Live Mattermost 11.7/12.x single-node and HA tests, native timed-DND expiration,
  desktop theme/keyboard behavior, and real server RPC behavior require the
  acceptance walkthrough in [UI documentation](ui.md). Mocked tests do not
  certify actual cluster failover or browser rendering.
- No separate formatter or linter run is claimed.

