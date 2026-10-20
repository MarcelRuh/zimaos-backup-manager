# Architektur

```
Browser → React SPA → REST/SSE → Go Backend → SQLite
                                 ↓
                         ZimaOS /v2/backup/*
                                 ↓
                              Discord Webhook
```

## Module

- `internal/zimaos` – HTTP-Client nur für bestätigte Endpunkte
- `internal/monitor` – Polling + Zustandsmaschine
- `internal/control` – Start-Preflight und Doppelstartschutz
- `internal/scheduler` – Cron ohne Missed-Job-Catch-up
- `internal/discord` – Embeds, Rate-Limit, Masking
- `internal/auth` – Setup, Sessions, CSRF, Argon2id
- `internal/api` – REST + SSE, Static SPA

## Datenpersistenz

Alles unter `DATA_DIR` (Default `/data`), inklusive SQLite `backup-manager.db`.
