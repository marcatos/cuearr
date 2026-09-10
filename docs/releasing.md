# Releasing Cuearr

GitHub Releases are produced by pushing an annotated tag matching `v*` (see `.github/workflows/release.yml`). Merging to `main` alone does **not** publish binaries or GHCR images.

## Agents / maintainers

Follow `.cursor/rules/releases.mdc`: after a roadmap batch or other shippable work reaches `main`, cut the next semver tag and push it. Default bump after a batch is **MINOR**.

## Manual steps

```bash
gh auth switch --user marcatos
git checkout main && git pull && git fetch --tags origin
git tag -a v0.X.Y -m "v0.X.Y — summary"
git push origin v0.X.Y
# watch: Actions → release
```

Images: `ghcr.io/marcatos/cuearr:v0.X.Y` and `:latest`.
