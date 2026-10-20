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

CasaOS/ZimaOS überspringen JWT-Prüfungen oft nur für `127.0.0.1` / `::1`. Vom Docker-Bridge-Netz aus kann ein `Authorization`-Token nötig sein.

## Live-Tests

Automatisierte Tests nutzen HTTP-Fixtures. End-to-End gegen eine reale ZimaCube-Instanz ist umgebungsabhängig und nicht Teil der CI.
