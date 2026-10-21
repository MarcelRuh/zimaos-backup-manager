import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button, Card, Input, Label, Select, SecondaryButton } from "@/components/ui";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import type { ComposeBackupHook, Schedule, Task } from "@/types";

type FormState = {
  taskId: string;
  name: string;
  preset: string;
  cron: string;
  enabled: boolean;
};

const emptyForm = (taskId = ""): FormState => ({
  taskId,
  name: "",
  preset: "daily",
  cron: "",
  enabled: true,
});

function matchPreset(cronExpr: string): string {
  switch (cronExpr) {
    case "0 3 * * *":
      return "daily";
    case "0 3 * * 1":
      return "weekly";
    case "0 3 1 * *":
      return "monthly";
    default:
      return "";
  }
}

export function SchedulesPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<Schedule[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [form, setForm] = useState<FormState>(emptyForm());
  const [editingId, setEditingId] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    const [schedules, taskList] = await Promise.all([
      api<Schedule[]>("/api/schedules"),
      api<Task[]>("/api/tasks"),
    ]);
    setItems(schedules || []);
    setTasks(taskList || []);
    setForm((prev) => {
      if (prev.taskId || !taskList?.[0]) return prev;
      return { ...prev, taskId: taskList[0].id };
    });
  }

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "error"));
  }, []);

  const selectedTask = tasks.find((task) => task.id === form.taskId);

  function hookValue(task: Task | undefined): ComposeBackupHook {
    const v = task?.prefs?.compose_backup_hook;
    if (v === "before" || v === "on_success" || v === "both") return v;
    return "off";
  }

  async function updateComposeHook(value: ComposeBackupHook) {
    if (!selectedTask) return;
    setError("");
    try {
      await api(`/api/tasks/${selectedTask.id}/prefs`, {
        method: "PUT",
        body: JSON.stringify({
          ...selectedTask.prefs,
          task_id: selectedTask.id,
          compose_backup_hook: value,
        }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  function beginEdit(sch: Schedule) {
    const preset = matchPreset(sch.cron_expr);
    setEditingId(sch.id);
    setForm({
      taskId: sch.task_id,
      name: sch.name || "",
      preset: preset || "daily",
      cron: preset ? "" : sch.cron_expr,
      enabled: sch.enabled,
    });
    setError("");
  }

  function cancelEdit() {
    setEditingId(null);
    setForm(emptyForm(tasks[0]?.id || ""));
    setError("");
  }

  async function save(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const body = {
        task_id: form.taskId,
        name: form.name,
        preset: form.cron ? "" : form.preset,
        cron_expr: form.cron || undefined,
        enabled: form.enabled,
      };
      if (editingId != null) {
        await api(`/api/schedules/${editingId}`, {
          method: "PUT",
          body: JSON.stringify(body),
        });
      } else {
        await api("/api/schedules", {
          method: "POST",
          body: JSON.stringify(body),
        });
      }
      cancelEdit();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  async function toggle(sch: Schedule) {
    setError("");
    try {
      await api(`/api/schedules/${sch.id}`, {
        method: "PUT",
        body: JSON.stringify({
          task_id: sch.task_id,
          name: sch.name,
          cron_expr: sch.cron_expr,
          timezone: sch.timezone,
          enabled: !sch.enabled,
        }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function remove(id: number) {
    setError("");
    try {
      await api(`/api/schedules/${id}`, { method: "DELETE" });
      if (editingId === id) cancelEdit();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  function hookLabel(taskIdValue: string) {
    const task = tasks.find((item) => item.id === taskIdValue);
    const hook = hookValue(task);
    switch (hook) {
      case "before":
        return t("backups.composeTimingBefore");
      case "on_success":
        return t("backups.composeTimingAfter");
      case "both":
        return t("backups.composeTimingBoth");
      default:
        return t("backups.composeTimingOff");
    }
  }

  function taskName(taskIdValue: string) {
    return tasks.find((task) => task.id === taskIdValue)?.name || taskIdValue;
  }

  return (
    <div className="space-y-5">
      <PageHeader title={t("schedules.title")} />
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      <Card>
        <h3 className="mb-4 text-lg font-semibold">
          {editingId != null ? t("schedules.edit") : t("schedules.create")}
        </h3>
        <form className="grid gap-3 md:grid-cols-2" onSubmit={(e) => void save(e)}>
          <div>
            <Label>{t("schedules.task")}</Label>
            <Select
              value={form.taskId}
              onChange={(e) => setForm({ ...form, taskId: e.target.value })}
              required
            >
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>{t("schedules.name")}</Label>
            <Input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="Nightly"
            />
          </div>
          <div>
            <Label>{t("schedules.preset")}</Label>
            <Select
              value={form.preset}
              onChange={(e) => setForm({ ...form, preset: e.target.value })}
              disabled={!!form.cron}
            >
              <option value="daily">daily</option>
              <option value="weekly">weekly</option>
              <option value="monthly">monthly</option>
            </Select>
          </div>
          <div>
            <Label>{t("schedules.cron")} (custom)</Label>
            <Input
              value={form.cron}
              onChange={(e) => setForm({ ...form, cron: e.target.value })}
              placeholder="0 3 * * *"
            />
          </div>
          <div className="md:col-span-2">
            <Label>{t("schedules.composeTiming")}</Label>
            <Select
              value={hookValue(selectedTask)}
              onChange={(e) => void updateComposeHook(e.target.value as ComposeBackupHook)}
              disabled={!selectedTask}
            >
              <option value="off">{t("backups.composeTimingOff")}</option>
              <option value="before">{t("backups.composeTimingBefore")}</option>
              <option value="on_success">{t("backups.composeTimingAfter")}</option>
              <option value="both">{t("backups.composeTimingBoth")}</option>
            </Select>
            <p className="mt-1 text-xs text-[var(--color-muted)]">{t("schedules.composeTimingHint")}</p>
          </div>
          {editingId != null && (
            <label className="flex items-center gap-2 text-sm md:col-span-2">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              />
              {t("schedules.enabled")}
            </label>
          )}
          <div className="flex flex-wrap gap-2 md:col-span-2">
            <Button type="submit" disabled={busy || !form.taskId}>
              {editingId != null ? t("schedules.save") : t("schedules.create")}
            </Button>
            {editingId != null && (
              <SecondaryButton type="button" onClick={cancelEdit}>
                {t("common.cancel")}
              </SecondaryButton>
            )}
          </div>
        </form>
      </Card>

      {items.length === 0 ? (
        <EmptyState>{t("schedules.empty")}</EmptyState>
      ) : (
        <div className="space-y-3">
          {items.map((sch) => (
            <Card
              key={sch.id}
              className={`flex flex-wrap items-center justify-between gap-3 ${
                editingId === sch.id ? "ring-1 ring-[var(--color-accent)]" : ""
              }`}
            >
              <div>
                <p className="font-semibold">{sch.name || sch.cron_expr}</p>
                <p className="mt-1 text-sm text-[var(--color-muted)]">{taskName(sch.task_id)}</p>
                <p className="mt-1 font-mono text-xs text-[var(--color-muted)]">
                  {sch.cron_expr} · {sch.timezone}
                  {sch.enabled ? "" : ` · ${t("schedules.disabled")}`}
                </p>
                <p className="mt-1 text-xs text-[var(--color-muted)]">
                  {t("schedules.composeTiming")}: {hookLabel(sch.task_id)}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <SecondaryButton type="button" onClick={() => beginEdit(sch)}>
                  {t("schedules.edit")}
                </SecondaryButton>
                <SecondaryButton type="button" onClick={() => void toggle(sch)}>
                  {sch.enabled ? t("schedules.disable") : t("schedules.enable")}
                </SecondaryButton>
                <SecondaryButton type="button" onClick={() => void remove(sch.id)}>
                  {t("schedules.delete")}
                </SecondaryButton>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
