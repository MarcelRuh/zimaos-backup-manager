import { useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { api, setCsrfToken } from "@/lib/api";
import { Button, Card, Input, Label } from "@/components/ui";

export function SetupPage({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const data = await api<{ csrf_token: string }>("/api/setup", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setCsrfToken(data.csrf_token);
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthShell title={t("auth.setupTitle")} hint={t("auth.setupHint")}>
      <form className="space-y-4" onSubmit={(e) => void submit(e)}>
        <div>
          <Label>{t("auth.username")}</Label>
          <Input value={username} onChange={(e) => setUsername(e.target.value)} required minLength={3} autoComplete="username" />
        </div>
        <div>
          <Label>{t("auth.password")}</Label>
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={10} autoComplete="new-password" />
        </div>
        {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
        <Button disabled={busy} type="submit" className="w-full">
          {t("auth.setup")}
        </Button>
      </form>
    </AuthShell>
  );
}

export function LoginPage({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const data = await api<{ csrf_token: string }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setCsrfToken(data.csrf_token);
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthShell title={t("auth.loginTitle")}>
      <form className="space-y-4" onSubmit={(e) => void submit(e)}>
        <div>
          <Label>{t("auth.username")}</Label>
          <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="username" />
        </div>
        <div>
          <Label>{t("auth.password")}</Label>
          <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" />
        </div>
        {error && <p className="text-sm text-[var(--color-err)]">{error}</p>}
        <Button disabled={busy} type="submit" className="w-full">
          {t("auth.login")}
        </Button>
      </form>
    </AuthShell>
  );
}

function AuthShell({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <div className="mx-auto flex min-h-screen max-w-md items-center px-4">
      <Card className="w-full">
        <p className="font-mono text-xs uppercase tracking-[0.18em] text-[var(--color-accent)]">ZimaOS</p>
        <h1 className="mt-2 text-2xl font-semibold">{t("appName")}</h1>
        <h2 className="mt-4 text-lg font-medium">{title}</h2>
        {hint && <p className="mt-2 text-sm text-[var(--color-muted)]">{hint}</p>}
        <div className="mt-6">{children}</div>
      </Card>
    </div>
  );
}
