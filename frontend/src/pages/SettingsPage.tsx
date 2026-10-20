import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button, Card, Input, Label, Select } from "@/components/ui";
import type { Settings, Task } from "@/types";
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
  const [token, setToken] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function load() {
    const [s, sys, logItems, taskList] = await Promise.all([
      api<Settings>("/api/settings"),
      api<SystemInfo>("/api/system"),
      api<LogItem[]>("/api/logs?limit=50"),
      api<Task[]>("/api/tasks"),
    ]);
    setSettings(s);
    setSystem(sys);
    setLogs(logItems || []);
    setTasks(taskList || []);
    onTheme((s.theme as ThemeMode) || "system");
    if (s.language && s.language !== i18n.language) {
      void i18n.changeLanguage(s.language);
      onLanguage(s.language);
    }
  }

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "error"));
  }, []);

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!settings) return;
    setError("");
    try {
      const body: Record<string, unknown> = {
        language: settings.language,
        theme: settings.theme,
        timezone: settings.timezone,
        zimaos_api_base_url: settings.zimaos_api_base_url,
        poll_interval_seconds: settings.poll_interval_seconds,
      };
      if (token) body.zimaos_api_token = token;
      const next = await api<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(body) });
      setSettings(next);
      setToken("");
      onTheme((next.theme as ThemeMode) || "system");
      void i18n.changeLanguage(next.language || "de");
      onLanguage(next.language || "de");
      setMessage("saved");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
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

  async function toggleTask(task: Task) {
    await api(`/api/tasks/${task.id}/prefs`, {
      method: "PUT",
      body: JSON.stringify({ ...task.prefs, task_id: task.id, enabled: !task.prefs.enabled }),
    });
    await load();
  }

  if (!settings) return <p className="text-[var(--color-muted)]">{t("common.loading")}</p>;

  return (
    <div className="space-y-5">
      <h2 className="text-2xl font-semibold">{t("settings.title")}</h2>
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
        <div className="space-y-2">
          {tasks.map((task) => (
            <label key={task.id} className="flex items-center justify-between gap-3 text-sm">
              <span>{task.name}</span>
              <input type="checkbox" checked={task.prefs?.enabled ?? true} onChange={() => void toggleTask(task)} />
            </label>
          ))}
        </div>
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
            <div key={log.id} className="rounded-lg border border-[var(--color-line)] px-3 py-2 text-xs">
              <span className="font-mono text-[var(--color-muted)]">{formatTs(log.created_at)}</span>{" "}
              <span className="font-semibold">[{log.level}/{log.category}]</span> {log.message}
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
