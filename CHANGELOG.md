# Changelog

## 0.1.7

- Compose→AppData: Docker-Label-Erkennung der Apps, Mapping in den richtigen AppData-Ordner
- Ablauf: ZimaOS-Backup erfolgreich → Compose nach AppData syncen → eigenes Discord-Embed
- Mounts: Docker-Socket, AppData (RW), CasaOS-Apps (RO); `APPDATA_HOST_ROOT` konfigurierbar

## 0.1.6

- Compose-Backup: Quellpfad automatisch erkennen oder in den Einstellungen wählen
- Compose-Backup: Speicherpfad der Archive wählbar (Standard `/data/casaos-apps-backups`, optional Mounts unter `/host`, `/mnt`, …)

## 0.1.5

- Optional CasaOS compose backup: archives `docker-compose.yml` from `/var/lib/casaos/apps` into `/data/casaos-apps-backups`
- Settings UI + cron schedule; RO bind mount documented in compose

## 0.1.4

- Dark-Neon UI im Proxora-Stil (Orbitron/Space Grotesk, Pink/Lila/Cyan)
- Branding: Logo, Favicon, ZimaOS App-Icon, README im Proxora-Layout
- Security: X-Forwarded-For nur mit `TRUSTED_PROXIES`; ZimaOS-Passwort AES-GCM at rest
- Discord-Retries respektieren Context; Setup-Rate-Limit; Schedule-Delete 404
- CI: `govulncheck`; Makefile nutzt `npm ci`

## 0.1.3

- Discord embeds match compact layout (icons, inline fields, HH:MM:SS, German footer)

## 0.1.2

- Auto-login against ZimaOS `POST /v1/users/login` when Docker gets HTTP 401
- Settings UI for ZimaOS username/password; token refresh without Bearer prefix

## 0.1.1

- Fix `/data` volume permission denied on ZimaOS/Docker by reclaiming ownership at container start, then dropping to non-root

## 0.1.0

- Erstes öffentliches Release
- Go-Backend mit SPA-Auslieferung
- ZimaOS Backup-Task-Erkennung und Start
- Monitoring-Zustandsmaschine, Historie, Zeitpläne
- Discord-Webhooks, Auth/Setup, Docker/GHCR/ZimaOS-Metadaten
