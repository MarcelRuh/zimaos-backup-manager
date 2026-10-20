import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, setCsrfToken } from "@/lib/api";
import { useTheme, type ThemeMode } from "@/hooks/useTheme";
import { Layout } from "@/components/Layout";
import { LoginPage, SetupPage } from "@/pages/AuthPages";
import { DashboardPage } from "@/pages/DashboardPage";
import { BackupsPage } from "@/pages/BackupsPage";
import { SchedulesPage } from "@/pages/SchedulesPage";
import { HistoryPage } from "@/pages/HistoryPage";
import { NotificationsPage } from "@/pages/NotificationsPage";
import { SettingsPage } from "@/pages/SettingsPage";

type AuthState = "loading" | "setup" | "login" | "ready";

export default function App() {
  const { i18n } = useTranslation();
  const { setMode } = useTheme("system");
  const [state, setState] = useState<AuthState>("loading");

  async function bootstrap() {
    try {
      const setup = await api<{ needs_setup: boolean }>("/api/setup/status");
      if (setup.needs_setup) {
        setState("setup");
        return;
      }
      const me = await api<{ csrf_token: string }>("/api/auth/me");
      setCsrfToken(me.csrf_token);
      setState("ready");
    } catch {
      setState("login");
    }
  }

  useEffect(() => {
    void bootstrap();
  }, []);

  useEffect(() => {
    if (state !== "ready") return;
    const es = new EventSource("/api/events", { withCredentials: true });
    const bump = () => window.dispatchEvent(new CustomEvent("zbm:live"));
    // Backend publishes named SSE as event:message with typed JSON payload.
    es.addEventListener("message", bump);
    es.addEventListener("connected", bump);
    return () => es.close();
  }, [state]);

  if (state === "loading") {
    return <div className="grid min-h-screen place-items-center text-[var(--color-muted)]">Loading…</div>;
  }
  if (state === "setup") {
    return <SetupPage onDone={() => setState("ready")} />;
  }
  if (state === "login") {
    return <LoginPage onDone={() => setState("ready")} />;
  }

  return (
    <Routes>
      <Route
        element={
          <Layout
            onLogout={() => {
              setCsrfToken("");
              setState("login");
            }}
          />
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="backups" element={<BackupsPage />} />
        <Route path="schedules" element={<SchedulesPage />} />
        <Route path="history" element={<HistoryPage />} />
        <Route path="notifications" element={<NotificationsPage />} />
        <Route
          path="settings"
          element={
            <SettingsPage
              onTheme={(mode: ThemeMode) => setMode(mode)}
              onLanguage={(lng) => void i18n.changeLanguage(lng)}
            />
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}
