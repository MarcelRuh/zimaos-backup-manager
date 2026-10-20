import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button, Card, Input, Label, Select, SecondaryButton } from "@/components/ui";
import type { Schedule, Task } from "@/types";

export function SchedulesPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<Schedule[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [taskId, setTaskId] = useState("");
  const [preset, setPreset] = useState("daily");
  const [cron, setCron] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  async function load() {
    const [schedules, taskList] = await Promise.all([
      api<Schedule[]>("/api/schedules"),
      api<Task[]>("/api/tasks"),
    ]);
    setItems(schedules || []);
    setTasks(taskList || []);
    if (!taskId && taskList?.[0]) setTaskId(taskList[0].id);
  }

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "error"));
  }, []);

  async function create(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await api("/api/schedules", {
        method: "POST",
        body: JSON.stringify({
          task_id: taskId,
          name,
          preset: cron ? "" : preset,
          cron_expr: cron || undefined,
          enabled: true,
        }),
      });
      setName("");
      setCron("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    }
  }

  async function toggle(sch: Schedule) {
    await api(`/api/schedules/${sch.id}`, {
      method: "PUT",
      body: JSON.stringify({ ...sch, enabled: !sch.enabled }),
    });
    await load();
  }

  async function remove(id: number) {
    await api(`/api/schedules/${id}`, { method: "DELETE" });
    await load();
  }

  return (
    <div className="space-y-5">
      <h2 className="text-2xl font-semibold">{t("schedules.title")}</h2>
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      <Card>
        <form className="grid gap-3 md:grid-cols-2" onSubmit={(e) => void create(e)}>
          <div>
            <Label>{t("schedules.task")}</Label>
            <Select value={taskId} onChange={(e) => setTaskId(e.target.value)} required>
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nightly" />
          </div>
          <div>
            <Label>Preset</Label>
            <Select value={preset} onChange={(e) => setPreset(e.target.value)} disabled={!!cron}>
              <option value="daily">daily</option>
              <option value="weekly">weekly</option>
              <option value="monthly">monthly</option>
            </Select>
          </div>
          <div>
            <Label>{t("schedules.cron")} (custom)</Label>
            <Input value={cron} onChange={(e) => setCron(e.target.value)} placeholder="0 3 * * *" />
          </div>
          <div className="md:col-span-2">
            <Button type="submit">{t("schedules.create")}</Button>
          </div>
        </form>
      </Card>

      {items.length === 0 ? (
        <Card>
          <p className="text-[var(--color-muted)]">{t("schedules.empty")}</p>
        </Card>
      ) : (
        <div className="space-y-3">
          {items.map((sch) => (
            <Card key={sch.id} className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-semibold">{sch.name || sch.cron_expr}</p>
                <p className="mt-1 font-mono text-xs text-[var(--color-muted)]">
                  task {sch.task_id} · {sch.cron_expr} · {sch.timezone}
                </p>
              </div>
              <div className="flex gap-2">
                <SecondaryButton onClick={() => void toggle(sch)}>
                  {sch.enabled ? "On" : "Off"}
                </SecondaryButton>
                <SecondaryButton onClick={() => void remove(sch.id)}>Delete</SecondaryButton>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
