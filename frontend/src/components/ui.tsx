import type { ButtonHTMLAttributes, InputHTMLAttributes, PropsWithChildren, SelectHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function Button({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center rounded-xl bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50 dark:text-[#042f2e]",
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
        "inline-flex items-center justify-center rounded-xl border border-[var(--color-line)] bg-[var(--color-surface)] px-4 py-2.5 text-sm font-semibold text-[var(--color-ink)] transition hover:bg-[color-mix(in_oklab,var(--color-surface),var(--color-ink)_4%)] disabled:opacity-50",
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
        "w-full rounded-xl border border-[var(--color-line)] bg-[var(--color-surface)] px-3 py-2.5 text-sm outline-none ring-[var(--color-accent)] focus:ring-2",
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
        "w-full rounded-xl border border-[var(--color-line)] bg-[var(--color-surface)] px-3 py-2.5 text-sm outline-none ring-[var(--color-accent)] focus:ring-2",
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
    <section
      className={cn(
        "rounded-2xl border border-[var(--color-line)] bg-[var(--color-surface)] p-5 shadow-[0_10px_30px_-24px_rgba(15,23,42,0.45)] animate-fade-up",
        className,
      )}
    >
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
  return <span className={cn("inline-flex rounded-full px-2.5 py-1 text-xs font-semibold", colors[tone])}>{label}</span>;
}

export function Label({ children }: PropsWithChildren) {
  return <label className="mb-1.5 block text-sm font-medium text-[var(--color-muted)]">{children}</label>;
}
