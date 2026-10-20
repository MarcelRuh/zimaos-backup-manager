import type { PropsWithChildren, ReactNode } from "react";

export function PageHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 className="zbm-title text-2xl">{title}</h2>
        {subtitle && <p className="mt-1 text-sm text-[var(--color-muted)]">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}

export function EmptyState({ children }: PropsWithChildren) {
  return (
    <div className="zbm-panel flex min-h-40 items-center justify-center p-8 text-center text-sm text-[var(--color-muted)]">
      {children}
    </div>
  );
}

export function ProgressBar({ value }: { value: number }) {
  const pct = Math.min(100, Math.max(0, value));
  return (
    <div className="zbm-gauge">
      <span style={{ width: `${pct}%` }} />
    </div>
  );
}
