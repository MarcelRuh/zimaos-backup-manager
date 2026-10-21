import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatBytes, formatTs, statusTone } from "@/lib/utils";
import { useLiveRefresh } from "@/hooks/useLiveRefresh";
import { Button, Card, Input, Label, Select, SecondaryButton, StatusPill } from "@/components/ui";
import { EmptyState, PageHeader, ProgressBar } from "@/components/PageHeader";
import type { ComposeBackupHook, ComposeBackupStatus, Settings, Task } from "@/types";

export function BackupsPage() {
  const { t } = useTranslation();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [composeStatus, setComposeStatus] = useState<ComposeBackupStatus | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [busyId, setBusyId] = useState("");
  const [composeBusy, setComposeBusy] = useState(false);
  const [pathsOpen, setPathsOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      const [taskList, s, compose] = await Promise.all([
        api<Task[]>("/api/tasks"),
        api<Settings>("/api/settings"),
        api<ComposeBackupStatus>("/api/compose-backup"),
      ]);
      setTasks(taskList || []);
      setSettings(s);
      setComposeStatus(compose);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    }
  }, []);

  useLiveRefresh(load, 4000);

  async function start(id: string) {
    setBusyId(id);
    setError("");
    setMessage("");
    try {
      await api(`/api/tasks/${id}/start`, { method: "POST" });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    } finally {
      setBusyId("");
    }
  }

  async function updateTaskPrefs(task: Task, patch: Partial<NonNullable<Task["prefs"]>>) {
    await api(`/api/tasks/${task.id}/prefs`, {
      method: "PUT",
      body: JSON.stringify({ ...task.prefs, ...patch, task_id: task.id }),
    });
    await load();
  }

  async function saveComposeSettings() {
    if (!settings) return;
    setError("");
    try {
      const next = await api<Settings>("/api/settings", {
        method: "PUT",
        body: JSON.stringify({
          compose_backup_enabled: settings.compose_backup_enabled,
          compose_backup_appdata_dir: settings.compose_backup_appdata_dir ?? "",
          compose_backup_appdata_host_root: settings.compose_backup_appdata_host_root ?? "",
          compose_backup_compose_dir: settings.compose_backup_compose_dir ?? "",
          compose_backup_compose_host_root: settings.compose_backup_compose_host_root ?? "",
        }),
      });
      setSettings(next);
      setMessage(t("common.save"));
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    }
  }

  async function runComposeBackup() {
    setComposeBusy(true);
    setError("");
    setMessage("");
    try {
      await api("/api/compose-backup/run", { method: "POST" });
      setMessage(t("backups.composeSynced"));
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    } finally {
      setComposeBusy(false);
    }
  }

  function hookValue(task: Task): ComposeBackupHook {
    const v = task.prefs?.compose_backup_hook;
    if (v === "before" || v === "on_success" || v === "both") return v;
    return "off";
  }

  return (
    <div className="space-y-5">
      <PageHeader title={t("backups.title")} />
      {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
      {message && <p className="text-sm text-[var(--color-ok)]">{message}</p>}

      {settings && (
        <Card>
          <h3 className="mb-2 text-lg font-semibold">{t("backups.composeTitle")}</h3>
          <p className="mb-4 text-sm text-[var(--color-muted)]">{t("backups.composeHint")}</p>
          <label className="mb-4 flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.compose_backup_enabled ?? true}
              onChange={(e) => setSettings({ ...settings, compose_backup_enabled: e.target.checked })}
            />
            {t("backups.composeEnabled")}
          </label>

          {composeStatus && (
            <dl className="mb-4 space-y-2 text-sm">
              <div className="flex justify-between gap-3">
                <dt className="text-[var(--color-muted)]">Docker</dt>
                <dd className="text-right">
                  {composeStatus.docker_available ? t("backups.composeDockerOk") : t("backups.composeDockerMissing")}
                  {!composeStatus.docker_available && composeStatus.docker_error ? (
                    <div className="mt-1 font-mono text-xs text-[var(--color-err)]">{composeStatus.docker_error}</div>
                  ) : null}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-[var(--color-muted)]">AppData</dt>
                <dd className="truncate font-mono text-xs">
                  {composeStatus.appdata_dir}
                  {composeStatus.appdata_available ? "" : ` · ${t("backups.composeMissing")}`}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-[var(--color-muted)]">{t("backups.composeProjects")}</dt>
                <dd>{composeStatus.project_count ?? 0}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-[var(--color-muted)]">{t("backups.composeLast")}</dt>
                <dd>
                  {composeStatus.last_ok_at ? (
                    <>
                      {formatTs(composeStatus.last_ok_at)}
                      {` · OK ${composeStatus.last_ok_count ?? 0}`}
                      {composeStatus.last_fail_count ? ` · Fail ${composeStatus.last_fail_count}` : ""}
                    </>
                  ) : (
                    "—"
                  )}
                </dd>
              </div>
              {composeStatus.last_error &&
              !(composeStatus.docker_available && /docker socket|docker ping/i.test(composeStatus.last_error)) ? (
                <div className="text-[var(--color-err)]">{composeStatus.last_error}</div>
              ) : null}
            </dl>
          )}

          <button
            type="button"
            className="mb-3 text-sm text-[var(--color-muted)] underline-offset-2 hover:underline"
            onClick={() => setPathsOpen((v) => !v)}
          >
            {pathsOpen ? t("backups.composePathsHide") : t("backups.composePathsShow")}
          </button>
          {pathsOpen && (
            <div className="mb-4 grid gap-3 md:grid-cols-2">
              <div>
                <Label>{t("backups.composeAppDataHostRoot")}</Label>
                <Input
                  className="font-mono text-xs"
                  value={settings.compose_backup_appdata_host_root || ""}
                  placeholder={composeStatus?.appdata_host_root || "/DATA/AppData"}
                  onChange={(e) => setSettings({ ...settings, compose_backup_appdata_host_root: e.target.value })}
                />
              </div>
              <div>
                <Label>{t("backups.composeAppDataDir")}</Label>
                <Input
                  className="font-mono text-xs"
                  value={settings.compose_backup_appdata_dir || ""}
                  placeholder={composeStatus?.appdata_dir || "/host/appdata"}
                  onChange={(e) => setSettings({ ...settings, compose_backup_appdata_dir: e.target.value })}
                />
              </div>
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <Button type="button" onClick={() => void saveComposeSettings()}>
              {t("common.save")}
            </Button>
            <SecondaryButton type="button" disabled={composeBusy} onClick={() => void runComposeBackup()}>
              {t("backups.composeRun")}
            </SecondaryButton>
          </div>

          {composeStatus?.projects?.length ? (
            <div className="mt-4">
              <p className="mb-2 text-sm font-medium">{t("backups.composeProjects")}</p>
              <ul className="space-y-1 font-mono text-xs text-[var(--color-muted)]">
                {composeStatus.projects.slice(0, 12).map((p) => (
                  <li key={p.name + p.compose_host} className="break-all">
                    {p.name} → AppData/{p.appdata_name}
                    {p.compose_readable ? "" : " · compose?"}
                    {p.appdata_writable ? "" : " · appdata?"}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </Card>
      )}

      {tasks.length === 0 ? (
        <EmptyState>{t("backups.empty")}</EmptyState>
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
            const running =
              statusTone(task.status) === "run" || task.subtasks?.some((s) => statusTone(s.status) === "run");
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
                    <ProgressBar value={progress} />
                  </div>
                )}
                <div className="mt-4">
                  <Label>{t("backups.composeTiming")}</Label>
                  <Select
                    value={hookValue(task)}
                    onChange={(e) =>
                      void updateTaskPrefs(task, { compose_backup_hook: e.target.value as ComposeBackupHook })
                    }
                  >
                    <option value="off">{t("backups.composeTimingOff")}</option>
                    <option value="before">{t("backups.composeTimingBefore")}</option>
                    <option value="on_success">{t("backups.composeTimingAfter")}</option>
                    <option value="both">{t("backups.composeTimingBoth")}</option>
                  </Select>
                  <p className="mt-1 text-xs text-[var(--color-muted)]">{t("backups.composeTimingHint")}</p>
                </div>
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
