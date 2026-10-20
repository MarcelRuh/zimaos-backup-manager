# ZimaOS Backup Manager

Zentrale Weboberfläche zur Verwaltung, Steuerung, Überwachung und Automatisierung **bereits vorhandener** ZimaOS-Backups auf ZimaCube / ZimaCube Pro.

Die Anwendung entwickelt **keine eigene Backup-Engine**. Sie nutzt ausschließlich die integrierte ZimaOS-Backup-API und verändert ZimaOS nicht.

## Funktionen

- Automatische Erkennung vorhandener Backup-Tasks
- Dashboard mit Statusübersicht und API-Verbindung
- Manuelles Starten von Backups inkl. Preflight-Checks
- Robuste Überwachung mit Zustandsmaschine (kein falscher Erfolg bei altem `completed`)
- Zeitpläne (täglich/wöchentlich/monatlich/Cron), Zeitzone konfigurierbar
- Discord-Webhook-Benachrichtigungen (Erfolg/Fehler)
- Persistente Historie in SQLite
- Sichere lokale Anmeldung (Argon2id, HTTP-only Cookies, CSRF, Brute-Force-Schutz)
- Dark/Light/System, Deutsch/Englisch
- Ein Container, Multi-Arch (`amd64`/`arm64`), GHCR-Release

## Screenshots

Platzhalter für Screenshots nach dem ersten Release unter `docs/screenshots/`.

## Systemanforderungen

- ZimaOS mit erreichbarer Backup-API
- Docker / Docker Compose
- Port `8787` (konfigurierbar)
- Schreibbares Datenvolume für SQLite

## Bestätigte ZimaOS-API

Nur diese Endpunkte werden verwendet:

```http
GET  /v2/backup/tasks
POST /v2/backup/tasks/{id}/start
```

Details und Einschränkungen: [docs/LIMITATIONS.md](docs/LIMITATIONS.md)

## Installation mit Docker Compose

1. Repository klonen oder Release-Compose verwenden.
2. Image: `ghcr.io/marcelruh/zimaos-backup-manager` (nach erstem Release verfügbar).
3. Starten:

```bash
cp .env.example .env
docker compose up -d
```

WebUI: `http://<host>:8787`

### Netzwerk zur ZimaOS-API

Im Container ist `localhost` **nicht** der Host. Die Compose-Dateien setzen:

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
environment:
  ZIMAOS_API_BASE_URL: http://host.docker.internal
```

Falls die API Auth verlangt (häufig, wenn Real-IP nicht `127.0.0.1` ist), Token in der WebUI oder als `ZIMAOS_API_TOKEN` setzen. Tokens erscheinen niemals unmaskiert in API-Responses.

## Installation auf ZimaOS

1. Image muss in GHCR (oder anderer Registry) veröffentlicht sein.
2. Compose mit `x-casaos`-Metadaten: [deploy/zimaos/docker-compose.yml](deploy/zimaos/docker-compose.yml)
3. Als Custom-App installieren bzw. später im App Store einreichen.

## Ersteinrichtung

1. WebUI öffnen
2. Administrator-Benutzername und Passwort festlegen (kein Standardpasswort)
3. Optional ZimaOS-API-URL/Token prüfen
4. Backups sollten automatisch erscheinen

## Discord

Unter **Benachrichtigungen**:

- Webhook-URL hinterlegen
- Global und je Task aktivieren
- Erfolg/Fehler getrennt steuern
- Testbenachrichtigung senden

Es werden **keine** Fortschrittsmeldungen gesendet. Pro eindeutigem Durchlauf maximal eine Abschlussmeldung.

## Backup-Verwaltung

- **Backups**: Karten mit Quelle/Ziel/Status/Fortschritt, Start-Button
- **Zeitpläne**: Cron oder Presets, aktivieren/deaktivieren
- **Historie**: Filter nach Task, Zeitraum, Ergebnis
- Doppelstarts und parallele Scheduler-Starts pro Task werden verhindert
- Verpasste Cron-Jobs werden **nicht** unkontrolliert nachgeholt

## Fehlerbehebung

| Symptom | Prüfung |
|---|---|
| Keine Tasks | API-URL, Token, Host-Gateway, Firewall |
| `api_unavailable` | `ZIMAOS_API_BASE_URL`, Token, Host-Erreichbarkeit |
| Start blockiert | Task läuft bereits / Ziel `is_connected=false` / Task deaktiviert |
| Discord ohne Nachricht | Webhook gesetzt? Global/Task-Flags? Nur Erfolg/Fehler, kein Progress |
| Datenbankfehler | Volume-Rechte auf `/data` |

Systemlogs: Einstellungen → System.

## Bekannte Einschränkungen

Siehe [docs/LIMITATIONS.md](docs/LIMITATIONS.md).

## Sicherheit

- Kein dauerhaftes Default-Passwort
- Argon2id-Passwort-Hashes
- HTTP-only Session-Cookies + CSRF
- Brute-Force-Lockout
- ZimaOS-API nur serverseitig
- Kein konfigurierbarer Open-Proxy
- Secrets (Token/Webhook) nicht im Frontend zurückgeben
- Siehe [SECURITY.md](SECURITY.md)

## Build für Entwickler

Voraussetzungen: Go 1.23+, Node 22+, Docker (optional)

```bash
# Backend-Tests
cd backend && go test ./...

# Frontend
cd frontend
npm install
npm test
npm run build   # schreibt nach backend/internal/static/dist

# Lokal starten
cd backend
set DATA_DIR=../data
set ZIMAOS_API_BASE_URL=http://127.0.0.1
go run ./cmd/server
```

Docker:

```bash
docker compose build
docker compose up
```

## Lizenz

MIT – siehe [LICENSE](LICENSE)
