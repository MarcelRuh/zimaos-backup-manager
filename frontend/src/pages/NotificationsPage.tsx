import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button, Card, Input, Label, SecondaryButton } from "@/components/ui";
import type { Settings, Task } from "@/types";

export function NotificationsPage() {
  const { t } = useTranslation();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [webhook, setWebhook] = useState("");
  const [tasks, setTasks] = useState<Task[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function load() {
    const [s, taskList] = await Promise.all([api<Settings>("/api/settings"), api<Task[]>("/api/tasks")]);
    setSettings(s);
    setTasks(taskList || []);
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
        discord_enabled: settings.discord_enabled,
        discord_notify_success: settings.discord_notify_success,
        discord_notify_failure: settings.discord_notify_failure,
      };
      if (webhook) body.discord_webhook_url = webhook;
      const next = await api<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(body) });
      setSettings(next);
      setWebhook("");
      setMessage("saved");
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function test() {
    setError("");
    try {
      await api("/api/discord/test", { method: "POST" });
      setMessage("test_ok");
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function savePrefs(task: Task, patch: Partial<Task["prefs"]>) {
    const prefs = { ...task.prefs, ...patch, task_id: task.id };
    await api(`/api/tasks/${task.id}/prefs`, { method: "PUT", body: JSON.stringify(prefs) });
    await load();
  }

  if (!settings) return <p className="text-[var(--color-muted)]">{t("common.loading")}</p>;

  return (
    <div className="space-y-5">
      <h2 className="text-2xl font-semibold">{t("notifications.title")}</h2>
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      {message && <p className="text-sm text-[var(--color-ok)]">{message}</p>}
      <Card>
        <form className="space-y-4" onSubmit={(e) => void save(e)}>
          <div>
            <Label>{t("notifications.webhook")}</Label>
            <Input
              type="password"
              value={webhook}
              onChange={(e) => setWebhook(e.target.value)}
              placeholder={settings.discord_webhook_set ? "•••••••• (gesetzt)" : "https://discord.com/api/webhooks/..."}
            />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.discord_enabled}
              onChange={(e) => setSettings({ ...settings, discord_enabled: e.target.checked })}
            />
            {t("notifications.enabled")}
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.discord_notify_success}
              onChange={(e) => setSettings({ ...settings, discord_notify_success: e.target.checked })}
            />
            {t("notifications.success")}
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.discord_notify_failure}
              onChange={(e) => setSettings({ ...settings, discord_notify_failure: e.target.checked })}
            />
            {t("notifications.failure")}
          </label>
          <div className="flex flex-wrap gap-2">
            <Button type="submit">{t("notifications.save")}</Button>
            <SecondaryButton type="button" onClick={() => void test()}>
              {t("notifications.test")}
            </SecondaryButton>
          </div>
        </form>
      </Card>

      <div className="space-y-3">
        {tasks.map((task) => (
          <Card key={task.id} className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="font-semibold">{task.name}</p>
              <p className="text-xs text-[var(--color-muted)]">Task {task.id}</p>
            </div>
            <div className="flex flex-wrap gap-3 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={task.prefs?.notify_enabled ?? true}
                  onChange={(e) => void savePrefs(task, { notify_enabled: e.target.checked })}
                />
                notify
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={task.prefs?.notify_success ?? true}
                  onChange={(e) => void savePrefs(task, { notify_success: e.target.checked })}
                />
                ok
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={task.prefs?.notify_failure ?? true}
                  onChange={(e) => void savePrefs(task, { notify_failure: e.target.checked })}
                />
                fail
              </label>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
