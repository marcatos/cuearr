# Cuearr

*arr-style daemon that splits **lossless image + CUE** albums into per-track files for Lidarr and Plex.

> Design (approved brainstorm): [`docs/superpowers/specs/2026-09-10-cuearr-design.md`](docs/superpowers/specs/2026-09-10-cuearr-design.md)

## Status

Pre-implementation. Spec under review. Tracking: **GitHub Issues** on [marcatos/cuearr](https://github.com/marcatos/cuearr).

## Build

Requires Go 1.23+.

```bash
go build -o bin/cuearr ./cmd/cuearr
```

On Windows:

```powershell
go build -o bin/cuearr.exe ./cmd/cuearr
```

## Run

```bash
./bin/cuearr version
./bin/cuearr serve
```

## Lidarr Connect

Cuearr accepts the same path on the webhook and on a custom script.

1. In Lidarr → **Settings → Connect**, add a **Webhook**.
   - URL: `http://<cuearr-host>:8787/api/v1/hooks/lidarr`
   - Method: **POST**
   - Header: `X-Api-Key` = your Cuearr API key (Settings in the UI, or bootstrap env).
   - Triggers: **On Download** and/or **On Import** (payload includes `environment.DownloadPath`).

2. Optional **Custom Script** (after import): copy [`scripts/lidarr-custom-script.sh`](scripts/lidarr-custom-script.sh) and point Lidarr at it.
   - Required env: `CUEARR_URL` (e.g. `http://cuearr:8787`), `CUEARR_API_KEY`.
   - Lidarr sets `lidarr_episodefile_path` to the imported file; the script posts `{"path":"..."}` to the same hook.

Response: `{"job_id":"<uuid>","created":true}` when a new split job is queued (`created:false` if the album was already queued or completed).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Use GitHub Issues and the provided templates for bugs and feature requests.

## Security

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities privately.

## License

MIT
