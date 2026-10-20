import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 1024) return `${n || 0} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let v = n;
  for (const u of units) {
    v /= 1024;
    if (v < 1024) return `${v.toFixed(2)} ${u}`;
  }
  return `${(v / 1024).toFixed(2)} PiB`;
}

export function formatTs(value?: string | number | null): string {
  if (value == null || value === "") return "—";
  const d = typeof value === "number" ? new Date(value * 1000) : new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}

export function statusTone(status: string): "ok" | "run" | "warn" | "err" | "idle" {
  const s = (status || "").toLowerCase();
  if (["completed", "complete", "success", "succeeded", "done", "finished"].includes(s)) return "ok";
  if (["running", "in_progress", "in-progress", "progressing", "active", "backing_up", "syncing"].includes(s)) return "run";
  if (["failed", "error", "errored", "cancelled", "canceled", "aborted"].includes(s)) return "err";
  if (["", "idle", "pending", "waiting", "manual", "none", "not_started", "never"].includes(s)) return "idle";
  return "warn";
}
