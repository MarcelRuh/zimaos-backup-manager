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
  return (
    <div className="mx-auto flex min-h-screen max-w-md items-center px-4">
      <Card className="w-full">
        <div className="flex flex-col items-center text-center">
          <img
            src="/brand/zimaos-backup-manager-logo.jpg"
            alt="ZimaOS Backup Manager"
            width={180}
            height={180}
            className="mb-4 h-auto w-[180px] drop-shadow-[0_0_28px_rgba(255,0,110,0.35)]"
          />
          <h2 className="text-lg font-medium">{title}</h2>
          {hint && <p className="mt-2 text-sm text-[var(--color-muted)]">{hint}</p>}
        </div>
        <div className="mt-6">{children}</div>
      </Card>
    </div>
  );
}
