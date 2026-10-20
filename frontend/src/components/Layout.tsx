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

  const brand = (
    <div className="mb-6 flex items-center gap-3">
      <img
        src="/brand/zimaos-backup-manager-icon.png"
        alt=""
        width={44}
        height={44}
        className="h-11 w-11 shrink-0 rounded-[4px] object-cover shadow-[0_0_20px_rgba(255,0,110,0.25)]"
      />
      <div className="min-w-0">
        <p className="zbm-logo-text text-sm leading-none">ZIMAOS</p>
        <p className="mt-1 truncate text-[11px] font-medium uppercase tracking-[0.16em] text-[var(--color-muted)]">
          Backup Manager
        </p>
      </div>
    </div>
  );

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
              "flex items-center gap-3 px-3 py-2.5 text-sm font-medium uppercase tracking-[0.05em] transition",
              isActive
                ? "zbm-nav-active"
                : "text-[var(--color-muted)] hover:bg-[var(--color-accent-soft)] hover:text-[var(--color-ink)]",
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
    <div className="mx-auto flex min-h-screen max-w-7xl gap-0 px-0 md:gap-0 md:px-0 lg:px-0">
      <aside className="hidden w-64 shrink-0 border-r border-[var(--color-line)] bg-[rgba(10,10,15,0.92)] md:block">
        <div className="sticky top-0 flex h-screen flex-col p-4">
          {brand}
          {nav}
          <SecondaryButton className="mt-auto w-full" onClick={() => void logout()}>
            {t("nav.logout")}
          </SecondaryButton>
        </div>
      </aside>

      <div className="min-w-0 flex-1 px-4 py-4 md:px-6 md:py-6">
        <header className="mb-4 flex items-center justify-between md:hidden">
          <div className="flex items-center gap-3">
            <img
              src="/brand/zimaos-backup-manager-icon.png"
              alt=""
              width={36}
              height={36}
              className="h-9 w-9 rounded-[4px] object-cover"
            />
            <div>
              <p className="zbm-logo-text text-sm">ZIMAOS</p>
              <p className="text-[10px] uppercase tracking-[0.16em] text-[var(--color-muted)]">Backup Manager</p>
            </div>
          </div>
          <button
            className="rounded-[4px] border border-[var(--color-line)] bg-[var(--color-surface)] p-2"
            onClick={() => setOpen((v) => !v)}
            aria-label="Menu"
          >
            {open ? <X size={18} /> : <Menu size={18} />}
          </button>
        </header>
        {open && (
          <div className="zbm-panel mb-4 p-4 md:hidden">
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
