# Verification

## Automated coverage

`docker compose run --rm builder release` passes in the Docker builder:

- Go tests with the race detector cover scheduling, Warsaw DST boundaries,
  persistence/restart recovery, overrides, concurrent KV decisions, executor
  changes, authenticated HTTP requests, and private slash commands.
- Eleven frontend tests cover validation, summaries, request revisions, errors,
  and the browser receiver required by native `fetch`.
- Six Playwright tests run Chromium against the compiled plugin bundle and the
  production Go HTTP handler. They cover settings registration, subpath routing,
  load/save/reload, multiple intervals, validation, concurrent edits, skip/reset,
  authentication failures, and backend failures. Browser `fetch` is not replaced.
- Twelve Python tests and six Node tests cover versioning, packaging, checksums,
  Git release races, and publishing through an injected GitHub client.
- TypeScript checking, Linux amd64/arm64 compilation, archive validation, and
  local checksum generation pass.

Run browser tests independently after dependency installation:

```sh
docker compose run --rm builder test-browser
```

The browser fixture uses React 17, a minimal implementation of Mattermost's
supported user-settings registration, and fake Mattermost KV/presence APIs.
Its HTTP server exists only in Go test code. Tests use isolated browser contexts
and a fixed server clock. They do not require a running Mattermost instance.

Browser tests are required by both CI and the manual release workflow. Failures
produce an HTML report, screenshots, and traces under `webapp/playwright-report/`
and `webapp/test-results/`; workflows upload these as diagnostic artifacts.
Playwright and Chromium versions must match between `webapp/package.json` and
`Dockerfile`.

## Live verification

Automated fixtures do not certify Mattermost's own settings renderer, RPC,
cluster failover, or native timed-DND expiration. Run the acceptance walkthrough
in [UI documentation](ui.md) on supported single-node and HA deployments before
publishing. Those live checks and execution of the updated workflows on GitHub
have not been verified in this workspace.
