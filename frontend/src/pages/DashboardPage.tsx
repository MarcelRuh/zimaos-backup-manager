import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatTs } from "@/lib/utils";
import { useLiveRefresh } from "@/hooks/useLiveRefresh";
import { Card, StatusPill } from "@/components/ui";
import type { BackupRun } from "@/types";

type Dash = {
  total_tasks: number;
  running: number;
  completed: number;
  failed: number;
  last_success: BackupRun | null;
  api_connected: boolean;
  api_last_ok?: string;
  api_last_error?: string;
};

export function DashboardPage() {
  const { t } = useTranslation();
  const [data, setData] = useState<Dash | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const d = await api<Dash>("/api/dashboard");
      setData(d);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "error");
    }
  }, []);

  useLiveRefresh(load, 5000);

  if (error) return <p className="text-[var(--color-err)]">{error}</p>;
  if (!data) return <p className="text-[var(--color-muted)]">{t("common.loading")}</p>;

  const stats = [
    { label: t("dashboard.total"), value: data.total_tasks },
    { label: t("dashboard.running"), value: data.running },
    { label: t("dashboard.completed"), value: data.completed },
    { label: t("dashboard.failed"), value: data.failed },
  ];

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-2xl font-semibold">{t("dashboard.title")}</h2>
          <p className="mt-1 text-sm text-[var(--color-muted)]">{t("dashboard.api")}</p>
        </div>
        <StatusPill
          tone={data.api_connected ? "ok" : "err"}
          label={data.api_connected ? t("dashboard.connected") : t("dashboard.disconnected")}
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {stats.map((s) => (
          <Card key={s.label}>
            <p className="text-sm text-[var(--color-muted)]">{s.label}</p>
            <p className="mt-2 font-mono text-3xl font-semibold tabular-nums">{s.value}</p>
          </Card>
        ))}
      </div>

      <Card>
        <p className="text-sm text-[var(--color-muted)]">{t("dashboard.lastSuccess")}</p>
        {data.last_success ? (
          <div className="mt-2">
            <p className="text-lg font-semibold">{data.last_success.backup_name}</p>
            <p className="mt-1 text-sm text-[var(--color-muted)]">
              {formatTs(data.last_success.ended_at || data.last_success.started_at)}
            </p>
          </div>
        ) : (
          <p className="mt-2 text-sm text-[var(--color-muted)]">—</p>
        )}
        {data.api_last_error && (
          <p className="mt-3 text-sm text-[var(--color-err)]">{data.api_last_error}</p>
        )}
      </Card>
    </div>
  );
}
