# Repository Guidelines

## Project Structure

- `server/`: Go plugin lifecycle, authenticated HTTP API, KV persistence, and scheduler; Go tests live beside implementation files.
- `webapp/src/`: React/TypeScript settings panel, API client, validation, and styles; frontend tests live in `webapp/tests/`.
- `scripts/`: bundle, license, version, and release tooling with Python and Node tests.
- `plugin.json`: plugin identity, version, compatibility, and executable paths.
- `docs/`: UI documentation, SDK references, and verification details.
- `.github/workflows/`: Docker-based CI and the manual release workflow. Generated archives belong in ignored `dist/` directories.

## Build, Test, and Development

Use Docker Compose; host Go, Node, npm, and Python installations are unnecessary.

```sh
export LOCAL_UID=$(id -u) LOCAL_GID=$(id -g) # Linux/WSL ownership
docker compose build builder
docker compose run --rm builder
```

The default run installs pinned dependencies, tests, type-checks, builds Linux amd64/arm64 binaries, and validates the plugin archive. After dependencies are installed:

- `docker compose run --rm builder test`: all tests and TypeScript checks.
- `docker compose run --rm builder test-server`: Go tests with the race detector.
- `docker compose run --rm builder release`: verified local bundle, checksum, and release notes; no publication.

## Coding Style

Use tabs in Go and four spaces in TypeScript and Python. Match existing lowercase source filenames, such as `service.go` and `settings.tsx`; use PascalCase for React components and exported Go identifiers. Python imports must be absolute. Keep TypeScript strict checking enabled. Do not run formatters or linters during agent work or introduce new tooling without agreement.

## Testing

Use Go's `testing` package (`*_test.go`, `Test...`), Node's test runner (`*.test.ts`, `test_publish.cjs`), and Python `unittest` (`test_*.py`). No numeric coverage threshold is configured. Cover affected behavior, especially timezone/DST boundaries, manual overrides, concurrency, authentication, and release failures. Prefer injected fakes over monkeypatching.

## Commits and Pull Requests

History uses concise imperative English subjects, for example, “Keep Go module metadata unchanged during release builds.” PR descriptions should explain behavior, validation, and limitations; link relevant issues and include screenshots for UI changes. Keep diffs focused. Create commits only when requested; every push requires explicit approval for those specific changes.

## Documentation

Keep README concise and factual. Exclude prompt fragments, LLM reasoning, conversation history, and narration of agent workflows from human-facing documentation.
