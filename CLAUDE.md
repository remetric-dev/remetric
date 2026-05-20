# Remetric — Claude Code instructions

## Project

Remetric is a static-binary doctor for self-hosted Prometheus/Grafana/Loki stacks. Full spec in `.claude/spec.md`. Active phase: see `docs/superpowers/specs/`.

## Hard rules

### Commit messages

- **Never** add `Co-Authored-By` trailers. Solo project, single-author attribution. Applies to single-line and multi-line messages.
- Match the existing style in `git log`. Imperative mood, no period at end of subject.

### Go code style

- Follow the **Google Go Style Guide** (https://google.github.io/styleguide/go/) for all new and modified Go code: decisions, best practices, naming, error handling, doc comments, package layout.
- When the `google-go-style` skill is available, invoke it before writing or modifying Go.
- Tooling before declaring Go work done — all must be clean:
  - **`make fmt`** → `gofumpt` (pinned `v0.7.0`) + `goimports -local github.com/remetric-dev/remetric`
  - **`make vet`** → `go vet ./...`
  - **`make lint`** → `golangci-lint run --timeout 5m`; uses local binary if installed, else pinned Docker image
  - **`make vuln`** → `govulncheck` (pinned `v1.3.0`)
- Do not use raw `gofmt` — use `gofumpt` via Makefile target.

### Test discipline

- TDD per Superpowers `test-driven-development` skill: write the failing test first, then the implementation.
- **No assertion libraries.** Use the stdlib idiom `if got != want { t.Errorf("Func(%v) = %v, want %v", in, got, want) }`. For structural diffs on complex types use `github.com/google/go-cmp/cmp` / `cmp.Diff`. This overrides the testify mention in `.claude/spec.md` §4 — Google Go Style takes precedence.
- Mock outbound HTTP with `net/http/httptest`. No `gomock`.
- Test doubles end in `Stub` / `Fake` / `Spy` / `Mock` per the style guide.

### Architectural anchors

- Read-only: never modify the target Prometheus or Grafana.
- Bounded concurrency to the target (default 5 in-flight requests; configurable).
- All env vars use the `REMETRIC_` prefix (the spec's `TSDB_*` examples are legacy — ignore them).

## License headers

Apache License 2.0 (see [LICENSE](LICENSE)). All hand-written `.go` files carry SPDX short-form header on the first two lines:

```
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik
```

Blank line separates header from `package` declaration (or package doc comment, which must remain immediately adjacent to `package`). New `.go` files include this header verbatim — same year, same name, same SPDX identifier. Auto-generated `.go` files (e.g. oapi-codegen output) are exempt.
