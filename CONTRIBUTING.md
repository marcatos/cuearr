# Contributing to Cuearr

Thank you for your interest in Cuearr.

## Project tracker

**GitHub Issues is the only project tracker** for bugs, features, and roadmap work. Please open or comment on an issue before large changes so we avoid duplicate effort.

Use the issue templates when reporting bugs, split failures, or feature requests. For design context, see the [design spec](docs/superpowers/specs/2026-09-10-cuearr-design.md).

## Assisted beta (B3)

Recruitment and reporting rules: **[docs/beta.md](docs/beta.md)**. Sign up with the **Beta tester signup** issue template; maintainers publish a biweekly summary on [#25](https://github.com/marcatos/cuearr/issues/25).

## Development

- **Go 1.23+** is required once the module lands.
- Run **`go test ./...`** before opening a pull request.
- Follow **[Conventional Commits](https://www.conventionalcommits.org/)** for commit messages (e.g. `feat(watcher): detect CUE sidecars`).

## What not to commit

- Never commit **`.env`** files, API keys, passwords, or other secrets.
- Never commit **real album dumps** or copyrighted audio. Use synthetic or minimal fixtures only.

## Pull requests

Fill out the PR template, link related issues, and keep changes focused. Security issues are handled separately; see [SECURITY.md](SECURITY.md).
