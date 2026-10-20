import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button, Card, Input, Label, Select, SecondaryButton } from "@/components/ui";
import { PageHeader } from "@/components/PageHeader";
import type { ComposeBackupHook, ComposeBackupStatus, Settings, Task } from "@/types";
import type { ThemeMode } from "@/hooks/useTheme";
import { formatTs } from "@/lib/utils";

type SystemInfo = {
  version: string;
  api_connected: boolean;
  api_last_ok?: string;
  api_last_error?: string;
  database_path: string;
  data_dir: string;
};

type LogItem = {
  id: number;
  level: string;
  category: string;
  message: string;
  created_at: string;
};

export function SettingsPage({
  onTheme,
  onLanguage,
}: {
  onTheme: (mode: ThemeMode) => void;
  onLanguage: (lng: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [logs, setLogs] = useState<LogItem[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [composeStatus, setComposeStatus] = useState<ComposeBackupStatus | null>(null);
  const [composeBusy, setComposeBusy] = useState(false);
  const [token, setToken] = useState("");
  const [zimaPassword, setZimaPassword] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function load() {
    const [s, sys, logItems, taskList, compose] = await Promise.all([
      api<Settings>("/api/settings"),
      api<SystemInfo>("/api/system"),
      api<LogItem[]>("/api/logs?limit=50"),
      api<Task[]>("/api/tasks"),
      api<ComposeBackupStatus>("/api/compose-backup"),
    ]);
    setSettings(s);
    setSystem(sys);
    setLogs(logItems || []);
    setTasks(taskList || []);
    setComposeStatus(compose);
    onTheme((s.theme as ThemeMode) || "system");
    if (s.language && s.language !== i18n.language) {
      void i18n.changeLanguage(s.language);
      onLanguage(s.language);
    }
  }

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "error"));
  }, []);

  async function saveSettings() {
    if (!settings) return;
    setError("");
    try {
      const body: Record<string, unknown> = {
        language: settings.language,
        theme: settings.theme,
        timezone: settings.timezone,
        zimaos_api_base_url: settings.zimaos_api_base_url,
        zimaos_username: settings.zimaos_username || "",
        poll_interval_seconds: settings.poll_interval_seconds,
        compose_backup_enabled: settings.compose_backup_enabled,
        compose_backup_cron: settings.compose_backup_cron || "0 3 * * *",
        compose_backup_appdata_dir: settings.compose_backup_appdata_dir ?? "",
        compose_backup_appdata_host_root: settings.compose_backup_appdata_host_root ?? "",
        compose_backup_compose_dir: settings.compose_backup_compose_dir ?? "",
        compose_backup_compose_host_root: settings.compose_backup_compose_host_root ?? "",
      };
      if (token) body.zimaos_api_token = token;
      if (zimaPassword) body.zimaos_password = zimaPassword;
      const next = await api<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(body) });
      setSettings(next);
      setToken("");
      setZimaPassword("");
      onTheme((next.theme as ThemeMode) || "system");
      void i18n.changeLanguage(next.language || "de");
      onLanguage(next.language || "de");
      setMessage("saved");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function save(e: FormEvent) {
    e.preventDefault();
    await saveSettings();
  }

  async function changePassword(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/api/auth/password", {
        method: "POST",
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      setCurrentPassword("");
      setNewPassword("");
      setMessage("password_changed");
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function updateTaskPrefs(task: Task, patch: Partial<Task["prefs"]>) {
    await api(`/api/tasks/${task.id}/prefs`, {
      method: "PUT",
      body: JSON.stringify({ ...task.prefs, ...patch, task_id: task.id }),
    });
    await load();
  }

  async function runComposeBackup() {
    setComposeBusy(true);
    setError("");
    try {
      await api("/api/compose-backup/run", { method: "POST" });
      setMessage("compose_backup_ok");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setComposeBusy(false);
    }
  }

  if (!settings) return <p className="text-[var(--color-muted)]">{t("common.loading")}</p>;

  return (
    <div className="space-y-5">
      <PageHeader title={t("settings.title")} />
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      {message && <p className="text-sm text-[var(--color-ok)]">{message}</p>}

      <Card>
        <h3 className="mb-4 text-lg font-semibold">{t("settings.general")}</h3>
        <form className="grid gap-3 md:grid-cols-2" onSubmit={(e) => void save(e)}>
          <div>
            <Label>{t("settings.language")}</Label>
            <Select
              value={settings.language}
              onChange={(e) => setSettings({ ...settings, language: e.target.value })}
            >
              <option value="de">Deutsch</option>
              <option value="en">English</option>
            </Select>
          </div>
          <div>
            <Label>{t("settings.theme")}</Label>
            <Select value={settings.theme} onChange={(e) => setSettings({ ...settings, theme: e.target.value })}>
              <option value="system">system</option>
              <option value="light">light</option>
              <option value="dark">dark</option>
            </Select>
          </div>
          <div>
            <Label>{t("settings.timezone")}</Label>
            <Input value={settings.timezone} onChange={(e) => setSettings({ ...settings, timezone: e.target.value })} />
          </div>
          <div>
            <Label>{t("settings.poll")}</Label>
            <Input
              type="number"
              min={2}
              max={300}
              value={settings.poll_interval_seconds}
              onChange={(e) => setSettings({ ...settings, poll_interval_seconds: Number(e.target.value) })}
            />
          </div>
          <div className="md:col-span-2">
            <Label>{t("settings.apiUrl")}</Label>
            <Input
              value={settings.zimaos_api_base_url}
              onChange={(e) => setSettings({ ...settings, zimaos_api_base_url: e.target.value })}
            />
            <p className="mt-2 text-xs text-[var(--color-muted)]">{t("settings.apiAuthHint")}</p>
          </div>
          <div>
            <Label>{t("settings.apiUser")}</Label>
            <Input
              value={settings.zimaos_username || ""}
              onChange={(e) => setSettings({ ...settings, zimaos_username: e.target.value })}
              autoComplete="username"
            />
          </div>
          <div>
            <Label>{t("settings.apiPassword")}</Label>
            <Input
              type="password"
              value={zimaPassword}
              onChange={(e) => setZimaPassword(e.target.value)}
              placeholder={settings.zimaos_password_set ? "•••••••• (gesetzt)" : ""}
              autoComplete="current-password"
            />
          </div>
          <div className="md:col-span-2">
            <Label>{t("settings.apiToken")}</Label>
            <Input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={settings.zimaos_api_token_set ? "•••••••• (gesetzt)" : ""}
            />
          </div>
          <div className="md:col-span-2">
            <Button type="submit">{t("settings.save")}</Button>
          </div>
        </form>
      </Card>

      <Card>
        <h3 className="mb-4 text-lg font-semibold">Backups</h3>
        <div className="space-y-4">
          {tasks.map((task) => (
            <div key={task.id} className="grid gap-2 border-b border-[var(--color-line)] pb-4 last:border-0 last:pb-0 md:grid-cols-[1fr_auto_minmax(12rem,16rem)] md:items-center">
              <span className="text-sm font-medium">{task.name}</span>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={task.prefs?.enabled ?? true}
                  onChange={() => void updateTaskPrefs(task, { enabled: !(task.prefs?.enabled ?? true) })}
                />
                {t("settings.taskEnabled")}
              </label>
              <div>
                <Label>{t("settings.composeBackupHook")}</Label>
                <Select
                  value={task.prefs?.compose_backup_hook === "on_success" ? "on_success" : "off"}
                  onChange={(e) =>
                    void updateTaskPrefs(task, { compose_backup_hook: e.target.value as ComposeBackupHook })
                  }
                >
                  <option value="off">{t("settings.composeBackupHookOff")}</option>
                  <option value="on_success">{t("settings.composeBackupHookOnSuccess")}</option>
                </Select>
              </div>
            </div>
          ))}
        </div>
      </Card>

      <Card>
        <h3 className="mb-2 text-lg font-semibold">{t("settings.composeBackup")}</h3>
        <p className="mb-4 text-sm text-[var(--color-muted)]">{t("settings.composeBackupHint")}</p>
        <div className="grid gap-3 md:grid-cols-2">
          <label className="flex items-center gap-2 text-sm md:col-span-2">
            <input
              type="checkbox"
              checked={settings.compose_backup_enabled ?? true}
              onChange={(e) => setSettings({ ...settings, compose_backup_enabled: e.target.checked })}
            />
            {t("settings.composeBackupEnabled")}
          </label>
          <div>
            <Label>{t("settings.composeBackupCron")}</Label>
            <Input
              value={settings.compose_backup_cron || "0 3 * * *"}
              onChange={(e) => setSettings({ ...settings, compose_backup_cron: e.target.value })}
            />
          </div>
          <div>
            <Label>{t("settings.composeAppDataHostRoot")}</Label>
            <Input
              className="font-mono text-xs"
              value={settings.compose_backup_appdata_host_root || ""}
              placeholder={composeStatus?.appdata_host_root || "/DATA/AppData"}
              onChange={(e) => setSettings({ ...settings, compose_backup_appdata_host_root: e.target.value })}
            />
          </div>
          <div>
            <Label>{t("settings.composeAppDataDir")}</Label>
            <Input
              className="font-mono text-xs"
              value={settings.compose_backup_appdata_dir || ""}
              placeholder={composeStatus?.appdata_dir || "/host/appdata"}
              onChange={(e) => setSettings({ ...settings, compose_backup_appdata_dir: e.target.value })}
            />
          </div>
        </div>
        {composeStatus && (
          <dl className="mt-4 space-y-2 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">Docker</dt>
              <dd>{composeStatus.docker_available ? t("settings.composeDockerOk") : t("settings.composeDockerMissing")}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">AppData</dt>
              <dd className="truncate font-mono text-xs">
                {composeStatus.appdata_dir}
                {composeStatus.appdata_available ? "" : ` · ${t("settings.composeBackupMissing")}`}
              </dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">{t("settings.composeProjects")}</dt>
              <dd>{composeStatus.project_count ?? 0}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">{t("settings.composeBackupLast")}</dt>
              <dd>
                {composeStatus.last_ok_at ? formatTs(composeStatus.last_ok_at) : "—"}
                {composeStatus.last_ok_count != null ? ` · OK ${composeStatus.last_ok_count}` : ""}
                {composeStatus.last_fail_count ? ` · Fail ${composeStatus.last_fail_count}` : ""}
              </dd>
            </div>
            {composeStatus.last_error && (
              <div className="text-[var(--color-err)]">{composeStatus.last_error}</div>
            )}
          </dl>
        )}
        <div className="mt-4 flex flex-wrap gap-2">
          <Button type="button" onClick={() => void saveSettings()}>
            {t("settings.save")}
          </Button>
          <SecondaryButton type="button" disabled={composeBusy} onClick={() => void runComposeBackup()}>
            {t("settings.composeBackupRun")}
          </SecondaryButton>
        </div>
        {composeStatus?.projects?.length ? (
          <div className="mt-4">
            <p className="mb-2 text-sm font-medium">{t("settings.composeProjects")}</p>
            <ul className="space-y-1 font-mono text-xs text-[var(--color-muted)]">
              {composeStatus.projects.slice(0, 12).map((p) => (
                <li key={p.name + p.compose_host}>
                  {p.name} → AppData/{p.appdata_name}
                  {p.compose_readable ? "" : " · compose?"}
                  {p.appdata_writable ? "" : " · appdata?"}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </Card>

      <Card>
        <h3 className="mb-4 text-lg font-semibold">{t("settings.password")}</h3>
        <form className="grid gap-3 md:grid-cols-2" onSubmit={(e) => void changePassword(e)}>
          <div>
            <Label>Current</Label>
            <Input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} required />
          </div>
          <div>
            <Label>New</Label>
            <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required minLength={10} />
          </div>
          <div className="md:col-span-2">
            <Button type="submit">{t("settings.password")}</Button>
          </div>
        </form>
      </Card>

      <Card>
        <h3 className="mb-4 text-lg font-semibold">{t("settings.system")}</h3>
        {system && (
          <dl className="space-y-2 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">{t("settings.version")}</dt>
              <dd className="font-mono">{system.version}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">API</dt>
              <dd>{system.api_connected ? "ok" : system.api_last_error || "down"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">Last OK</dt>
              <dd>{formatTs(system.api_last_ok)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--color-muted)]">DB</dt>
              <dd className="truncate font-mono text-xs">{system.database_path}</dd>
            </div>
          </dl>
        )}
        <div className="mt-4 max-h-64 space-y-2 overflow-auto">
          {logs.map((log) => (
            <div key={log.id} className="rounded-[4px] border border-[var(--color-line)] bg-[rgba(0,0,0,0.2)] px-3 py-2 text-xs">
              <span className="font-mono text-[var(--color-muted)]">{formatTs(log.created_at)}</span>{" "}
              <span className="font-semibold">[{log.level}/{log.category}]</span> {log.message}
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
