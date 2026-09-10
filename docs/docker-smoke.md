# Cuearr release smoke (Docker)

Use after a tagged release image exists on GHCR.

## Prerequisites

- Docker Engine running
- Ports: `8787` free
- Optional: `shntool` only needed on the **host** if you split outside the container; the image already includes it

## Steps

```bash
cd deploy/docker
export CUEARR_INITIAL_PASSWORD='smoke-test-password'
docker compose pull   # or: docker compose up -d --build
docker compose up -d
curl -fsS "http://127.0.0.1:8787/api/v1/health"
```

Expected: JSON health with process up (and splitter available inside the image).

1. Open http://127.0.0.1:8787 — log in with the bootstrap password.
2. Confirm Settings show watch `/watch` and out `/out`.
3. Drop a synthetic album (see `scripts/generate_fixture.sh`) into `deploy/docker/watch/`.
4. Dashboard should show a job → `completed`; tracks appear under `deploy/docker/out/<album-subdir>/`.

```bash
docker compose down
```

Tracked as [#14](https://github.com/marcatos/cuearr/issues/14).
