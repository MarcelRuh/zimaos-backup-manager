# Changelog\n\n## 0.1.3\n\n- Discord embeds match compact layout (icons, inline fields, HH:MM:SS, German footer)\n\n## 0.1.2

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
