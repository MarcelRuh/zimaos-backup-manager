import { NavLink, Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import {
  Bell,
  CalendarClock,
  HardDrive,
  History,
  LayoutDashboard,
  Menu,
  Settings,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { SecondaryButton } from "@/components/ui";
import { api } from "@/lib/api";

const links = [
  { to: "/", key: "dashboard", icon: LayoutDashboard },
  { to: "/backups", key: "backups", icon: HardDrive },
  { to: "/schedules", key: "schedules", icon: CalendarClock },
  { to: "/history", key: "history", icon: History },
  { to: "/notifications", key: "notifications", icon: Bell },
  { to: "/settings", key: "settings", icon: Settings },
] as const;

export function Layout({ onLogout }: { onLogout: () => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  async function logout() {
    try {
      await api("/api/auth/logout", { method: "POST" });
    } finally {
      onLogout();
    }
  }

  const nav = (
    <nav className="flex flex-col gap-1">
      {links.map(({ to, key, icon: Icon }) => (
        <NavLink
          key={to}
          to={to}
          end={to === "/"}
          onClick={() => setOpen(false)}
          className={({ isActive }) =>
            cn(
              "flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition",
              isActive
                ? "bg-[var(--color-accent-soft)] text-[var(--color-accent)]"
                : "text-[var(--color-muted)] hover:bg-[color-mix(in_oklab,var(--color-surface),var(--color-ink)_4%)] hover:text-[var(--color-ink)]",
            )
          }
        >
          <Icon size={18} />
          {t(`nav.${key}`)}
        </NavLink>
      ))}
    </nav>
  );

  return (
    <div className="mx-auto flex min-h-screen max-w-7xl gap-6 px-4 py-4 md:px-6 md:py-6">
      <aside className="hidden w-64 shrink-0 md:block">
        <div className="sticky top-6 rounded-2xl border border-[var(--color-line)] bg-[var(--color-surface)] p-4 shadow-[0_10px_30px_-24px_rgba(15,23,42,0.45)]">
          <div className="mb-6">
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-[var(--color-accent)]">ZimaOS</p>
            <h1 className="mt-1 text-lg font-semibold leading-tight">{t("appName")}</h1>
          </div>
          {nav}
          <SecondaryButton className="mt-6 w-full" onClick={() => void logout()}>
            {t("nav.logout")}
          </SecondaryButton>
        </div>
      </aside>

      <div className="min-w-0 flex-1">
        <header className="mb-4 flex items-center justify-between md:hidden">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-[var(--color-accent)]">ZimaOS</p>
            <h1 className="text-base font-semibold">{t("appName")}</h1>
          </div>
          <button
            className="rounded-xl border border-[var(--color-line)] bg-[var(--color-surface)] p-2"
            onClick={() => setOpen((v) => !v)}
            aria-label="Menu"
          >
            {open ? <X size={18} /> : <Menu size={18} />}
          </button>
        </header>
        {open && (
          <div className="mb-4 rounded-2xl border border-[var(--color-line)] bg-[var(--color-surface)] p-4 md:hidden">
            {nav}
            <SecondaryButton className="mt-4 w-full" onClick={() => void logout()}>
              {t("nav.logout")}
            </SecondaryButton>
          </div>
        )}
        <Outlet />
      </div>
    </div>
  );
}
