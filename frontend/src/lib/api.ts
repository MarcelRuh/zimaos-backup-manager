export type ApiEnvelope<T> = {
  success: boolean;
  data: T;
  error: string;
};

let csrfToken = "";

export function setCsrfToken(token: string) {
  csrfToken = token;
}

export function getCsrfToken() {
  return csrfToken;
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (init.method && init.method !== "GET" && init.method !== "HEAD") {
    headers.set("X-CSRF-Token", csrfToken);
  }
  const res = await fetch(path, { ...init, headers, credentials: "include" });
  const json = (await res.json()) as ApiEnvelope<T>;
  if (!res.ok || !json.success) {
    throw new Error(json.error || `http_${res.status}`);
  }
  return json.data;
}
