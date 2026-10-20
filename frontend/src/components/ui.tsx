import type { ButtonHTMLAttributes, InputHTMLAttributes, PropsWithChildren, SelectHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function Button({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={cn(
        "zbm-btn-primary zbm-press inline-flex items-center justify-center rounded-[4px] px-4 py-2.5 text-sm font-semibold transition hover:opacity-90 focus-visible:outline-none focus-visible:shadow-[var(--ui-focus-glow)] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export function SecondaryButton({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={cn(
        "zbm-press inline-flex items-center justify-center rounded-[4px] border border-[var(--color-line)] bg-[var(--color-surface)] px-4 py-2.5 text-sm font-semibold text-[var(--color-ink)] transition hover:border-[rgba(131,56,236,0.45)] hover:bg-[var(--color-accent-soft)] focus-visible:outline-none focus-visible:shadow-[var(--ui-focus-glow)] disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "w-full rounded-[4px] border border-[rgba(131,56,236,0.45)] bg-[var(--color-surface)] px-3 py-2.5 text-sm text-[var(--color-ink)] outline-none focus:shadow-[var(--ui-focus-glow)]",
        className,
      )}
      {...props}
    />
  );
}

export function Select({ className, children, ...props }: PropsWithChildren<SelectHTMLAttributes<HTMLSelectElement>>) {
  return (
    <select
      className={cn(
        "w-full rounded-[4px] border border-[rgba(131,56,236,0.45)] bg-[var(--color-surface)] px-3 py-2.5 text-sm text-[var(--color-ink)] outline-none focus:shadow-[var(--ui-focus-glow)]",
        className,
      )}
      {...props}
    >
      {children}
    </select>
  );
}

export function Card({ children, className }: PropsWithChildren<{ className?: string }>) {
  return (
    <section className={cn("zbm-panel animate-fade-up p-5", className)}>
      {children}
    </section>
  );
}

export function StatusPill({ tone, label }: { tone: "ok" | "run" | "warn" | "err" | "idle"; label: string }) {
  const colors = {
    ok: "bg-[color-mix(in_oklab,var(--color-ok)_18%,transparent)] text-[var(--color-ok)]",
    run: "bg-[color-mix(in_oklab,var(--color-run)_18%,transparent)] text-[var(--color-run)]",
    warn: "bg-[color-mix(in_oklab,var(--color-warn)_18%,transparent)] text-[var(--color-warn)]",
    err: "bg-[color-mix(in_oklab,var(--color-err)_18%,transparent)] text-[var(--color-err)]",
    idle: "bg-[color-mix(in_oklab,var(--color-idle)_18%,transparent)] text-[var(--color-idle)]",
  };
  return (
    <span className={cn("inline-flex rounded-[4px] px-2.5 py-1 text-xs font-semibold uppercase tracking-wide", colors[tone])}>
      {label}
    </span>
  );
}

export function Label({ children }: PropsWithChildren) {
  return <label className="mb-1.5 block text-sm font-medium text-[var(--color-muted)]">{children}</label>;
}
