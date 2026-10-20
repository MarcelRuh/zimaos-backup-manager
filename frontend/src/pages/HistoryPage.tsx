import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatBytes, formatTs, statusTone } from "@/lib/utils";
import { Card, Select, StatusPill } from "@/components/ui";
import type { BackupRun, Task } from "@/types";

export function HistoryPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<BackupRun[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [taskId, setTaskId] = useState("");
  const [result, setResult] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    void api<Task[]>("/api/tasks").then(setTasks).catch(() => undefined);
  }, []);

  useEffect(() => {
    const q = new URLSearchParams();
    if (taskId) q.set("task_id", taskId);
    if (result) q.set("result", result);
    void api<{ items: BackupRun[] }>(`/api/history?${q}`)
      .then((d) => setItems(d.items || []))
      .catch((e) => setError(e instanceof Error ? e.message : "error"));
  }, [taskId, result]);

  return (
    <div className="space-y-5">
      <h2 className="text-2xl font-semibold">{t("history.title")}</h2>
      <Card className="grid gap-3 md:grid-cols-2">
        <Select value={taskId} onChange={(e) => setTaskId(e.target.value)}>
          <option value="">{t("history.filterBackup")}: {t("history.all")}</option>
          {tasks.map((task) => (
            <option key={task.id} value={task.id}>
              {task.name}
            </option>
          ))}
        </Select>
        <Select value={result} onChange={(e) => setResult(e.target.value)}>
          <option value="">{t("history.filterResult")}: {t("history.all")}</option>
          <option value="success">{t("history.success")}</option>
          <option value="failed">{t("history.failed")}</option>
          <option value="unknown">{t("history.unknown")}</option>
          <option value="ambiguous">ambiguous</option>
        </Select>
      </Card>
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      <Card className="overflow-x-auto p-0">
        <table className="min-w-full text-left text-sm">
          <thead className="border-b border-[var(--color-line)] text-[var(--color-muted)]">
            <tr>
              <th className="px-4 py-3 font-medium">Backup</th>
              <th className="px-4 py-3 font-medium">Result</th>
              <th className="px-4 py-3 font-medium">{t("history.duration")}</th>
              <th className="px-4 py-3 font-medium">Files</th>
              <th className="px-4 py-3 font-medium">Size</th>
              <th className="px-4 py-3 font-medium">Start</th>
            </tr>
          </thead>
          <tbody>
            {items.map((run) => (
              <tr key={run.id} className="border-b border-[var(--color-line)] last:border-0">
                <td className="px-4 py-3">
                  <div className="font-medium">{run.backup_name}</div>
                  <div className="text-xs text-[var(--color-muted)]">{run.error_message}</div>
                </td>
                <td className="px-4 py-3">
                  <StatusPill tone={statusTone(run.result === "success" ? "completed" : run.result === "failed" ? "failed" : run.result === "running" ? "running" : "")} label={run.result} />
                </td>
                <td className="px-4 py-3 font-mono">
                  {run.duration_ms != null ? `${Math.round(run.duration_ms / 1000)}s` : "—"}
                  <div className="text-xs text-[var(--color-muted)]">{run.duration_quality}</div>
                </td>
                <td className="px-4 py-3 font-mono">
                  {run.xferred_files}/{run.total_files}
                </td>
                <td className="px-4 py-3 font-mono">
                  {formatBytes(run.xferred_size)}/{formatBytes(run.total_size)}
                </td>
                <td className="px-4 py-3">{formatTs(run.started_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}
