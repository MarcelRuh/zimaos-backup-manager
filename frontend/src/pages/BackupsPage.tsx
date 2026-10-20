import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatBytes, formatTs, statusTone } from "@/lib/utils";
import { Button, Card, StatusPill } from "@/components/ui";
import type { Task } from "@/types";

export function BackupsPage() {
  const { t } = useTranslation();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");

  async function load() {
    try {
      setTasks(await api<Task[]>("/api/tasks"));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    }
  }

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 4000);
    return () => window.clearInterval(id);
  }, []);

  async function start(id: string) {
    setBusyId(id);
    try {
      await api(`/api/tasks/${id}/start`, { method: "POST" });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    } finally {
      setBusyId("");
    }
  }

  return (
    <div className="space-y-5">
      <h2 className="text-2xl font-semibold">{t("backups.title")}</h2>
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      {tasks.length === 0 ? (
        <Card>
          <p className="text-[var(--color-muted)]">{t("backups.empty")}</p>
        </Card>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {tasks.map((task) => {
            const target = task.subtasks?.[0]?.target;
            const totalFiles = task.subtasks?.reduce((a, s) => a + (s.total_files || 0), 0) || 0;
            const xFiles = task.subtasks?.reduce((a, s) => a + (s.xferred_files || 0), 0) || 0;
            const totalSize = task.subtasks?.reduce((a, s) => a + (s.total_size || 0), 0) || 0;
            const xSize = task.subtasks?.reduce((a, s) => a + (s.xferred_size || 0), 0) || 0;
            const progress =
              task.subtasks && task.subtasks.length
                ? task.subtasks.reduce((a, s) => a + (s.progress || 0), 0) / task.subtasks.length
                : 0;
            const running = statusTone(task.status) === "run" || task.subtasks?.some((s) => statusTone(s.status) === "run");
            return (
              <Card key={task.id}>
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h3 className="text-lg font-semibold">{task.name}</h3>
                    <p className="mt-1 font-mono text-xs text-[var(--color-muted)]">ID {task.id}</p>
                  </div>
                  <StatusPill tone={statusTone(task.status)} label={task.status || "unknown"} />
                </div>
                <dl className="mt-4 space-y-2 text-sm">
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--color-muted)]">{t("backups.source")}</dt>
                    <dd className="text-right">{task.source?.path || "—"}</dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--color-muted)]">{t("backups.target")}</dt>
                    <dd className="text-right">{target?.path || "—"}</dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--color-muted)]">{t("backups.last")}</dt>
                    <dd>{formatTs(task.last_backup_time)}</dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--color-muted)]">{t("backups.files")}</dt>
                    <dd className="font-mono">
                      {xFiles} / {totalFiles}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[var(--color-muted)]">{t("backups.size")}</dt>
                    <dd className="font-mono">
                      {formatBytes(xSize)} / {formatBytes(totalSize)}
                    </dd>
                  </div>
                </dl>
                {running && (
                  <div className="mt-4">
                    <div className="mb-1 flex justify-between text-xs text-[var(--color-muted)]">
                      <span>{t("backups.progress")}</span>
                      <span className="font-mono">{Math.round(progress)}%</span>
                    </div>
                    <div className="h-2 overflow-hidden rounded-full bg-[var(--color-line)]">
                      <div
                        className="h-full rounded-full bg-[var(--color-run)] transition-all duration-500"
                        style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
                      />
                    </div>
                  </div>
                )}
                <Button
                  className="mt-5 w-full"
                  disabled={busyId === task.id || running || task.prefs?.enabled === false}
                  onClick={() => void start(task.id)}
                >
                  {t("backups.start")}
                </Button>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
