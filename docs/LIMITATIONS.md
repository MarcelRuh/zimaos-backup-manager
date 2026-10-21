# Bekannte Einschränkungen

## ZimaOS-API

Öffentlich dokumentierte IceWhale-OpenAPI-Specs enthalten **keinen** Backup-Service unter `/v2/backup/*`.

Die folgenden Endpunkte wurden vom Projektauftraggeber auf einem ZimaCube Pro erfolgreich getestet und sind die **einzigen** verwendeten Backup-Schnittstellen:

- `GET /v2/backup/tasks`
- `POST /v2/backup/tasks/{id}/start`

Es werden bewusst **keine** weiteren Backup-Endpunkte angenommen (Stop, Pause, Einzel-Run-Historie, Restore usw.).

## Überwachung ohne Run-ID

Die API liefert keinen bestätigten eindeutigen Identifier pro Backup-Durchlauf.

Der Manager verwendet daher:

- Baseline von `status` / `last_backup_time` vor/bei Beobachtungsbeginn
- Beobachtung von Running-Zuständen
- Erst danach Erfolgs-/Fehlerbewertung

Ein alter `completed`-Status ohne Running-Phase und ohne fortgeschrittenes `last_backup_time` wird **niemals** als Erfolg eines neuen Starts gewertet.

Wenn der Abschluss nicht zweifelsfrei ist, speichert die Historie `unknown` oder `ambiguous` – nicht `success`.

## Laufzeiten

Wenn die API keine exakte Startzeit liefert, wird die Dauer als `estimated` gekennzeichnet. Nach beobachteter Running-Phase gilt sie als `measured` (lokal gestoppt).

## Auth vom Container aus

CasaOS/ZimaOS überspringen JWT-Prüfungen oft nur für `127.0.0.1` / `::1`. Vom Docker-Bridge-Netz (`host.docker.internal`) kommt typischerweise `401 Unauthorized`.

Bestätigter Login-Endpunkt:

```http
POST /v1/users/login
```

Antwort enthält `data.token.access_token` (temporär; keine permanenten API-Keys laut IceWhale).

Der Backup Manager speichert optional ZimaOS-Benutzername/Passwort und holt bei `401` automatisch ein neues Token. Das Token wird **ohne** `Bearer `-Prefix im `Authorization`-Header gesendet.

Das ZimaOS-Passwort liegt in SQLite nur AES-GCM-verschlüsselt (`enc:v1:…`); der Schlüssel liegt separat unter `DATA_DIR/.encryption-key` (Mode 0600). Verlust der Key-Datei macht gespeicherte Passwörter unlesbar – dann neu setzen.

## Compose → AppData (nach ZimaOS-Backup)

Nach **erfolgreichem** ZimaOS-Backup (wenn am Task `compose_backup_hook=on_success` gesetzt ist) erkennt der Manager Docker-Compose-Projekte über Labels (`com.docker.compose.project.config_files`) und schreibt `docker-compose.yml` in den zugehörigen AppData-Ordner.

Voraussetzungen (Compose-Mounts):

- Docker-Socket (`/var/run/docker.sock`) — muss als Bind-Mount vorhanden sein
- AppData RW (z. B. `/DATA/AppData` → `/host/appdata`)
- CasaOS-Apps RO (z. B. `/var/lib/casaos/apps` → `/host/casaos/apps`)
- `APPDATA_HOST_ROOT` muss dem Pfad entsprechen, der in `docker inspect` Mounts erscheint (z. B. `/DATA/AppData` oder `/media/SSD-Storage/AppData`)

Fehler `compose_backup_docker_missing`: Socket fehlt (kein Bind-Mount). Mit gemountetem Socket startet der Prozess als **root**, damit Compose-Dateien in typischerweise root-eigene AppData-Ordner geschrieben werden können.

`compose?` in der UI: Quell-`docker-compose.yml` im Container nicht lesbar (CasaOS-Apps-Mount oder Label-Pfad). Status zeigt dann Host- und Local-Pfad.

Sicherheit: Socket-Zugriff bedeutet volle Docker-API auf dem Host (effektiv root-äquivalent). Nur auf vertrauenswürdigen Homelab-Hosts mounten.

Das ist **kein** Ersatz für das ZimaOS-AppData-Backup selbst und stoppt keine Container. Discord sendet ein **eigenes** Embed für den Compose-Lauf.

## Live-Tests

Automatisierte Tests nutzen HTTP-Fixtures. End-to-End gegen eine reale ZimaCube-Instanz ist umgebungsabhängig und nicht Teil der CI.
