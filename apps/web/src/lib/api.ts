/**
 * The web client's only doorway to the Go monolith.
 *
 * Every call is defensive by design: the API is optional at render time, so a
 * missing or unreachable backend degrades to seeded fixtures rather than an
 * error page (see lib/content.ts). Mutations carry the CSRF double-submit
 * token read from the `alexandria_csrf` cookie, which the API binds to the
 * session token.
 */

const SERVER_BASE = (process.env.ALEXANDRIA_API_URL ?? "").replace(/\/$/, "");
/** Browser calls go through the same-origin rewrite (see next.config.mjs). */
const CLIENT_BASE = "/api";

export const apiConfigured = () => SERVER_BASE !== "";
export const clientConfigured = () => true; // the rewrite 404s cleanly if the API is absent

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Opts = {
  /** Server components forward the request's Cookie header so sessions work. */
  cookie?: string;
  csrf?: string;
  signal?: AbortSignal;
};

function readCsrfCookie(): string | null {
  if (typeof document === "undefined") return null;
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

async function request<T>(
  method: "GET" | "POST" | "PATCH" | "DELETE",
  path: string,
  body: unknown,
  opts: Opts,
): Promise<T> {
  const isServer = typeof window === "undefined";
  const base = isServer ? SERVER_BASE : CLIENT_BASE;
  if (isServer && !apiConfigured()) throw new ApiError(0, "no_api", "API not configured");
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.cookie) headers.Cookie = opts.cookie;
  if (method !== "GET") {
    const csrf = opts.csrf ?? readCsrfCookie();
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }
  const res = await fetch(`${base}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: opts.signal,
    // Sessions are cookie-based; never send them cross-origin.
    credentials: "same-origin",
    cache: "no-store",
  });
  const text = await res.text();
  const json = text ? (JSON.parse(text) as any) : {};
  if (!res.ok) {
    const err = json?.error ?? {};
    throw new ApiError(res.status, err.code ?? "http", err.message ?? res.statusText);
  }
  return json as T;
}

export const api = {
  get: <T>(path: string, opts: Opts = {}) => request<T>("GET", path, undefined, opts),
  post: <T>(path: string, body: unknown, opts: Opts = {}) => request<T>("POST", path, body, opts),
  patch: <T>(path: string, body: unknown, opts: Opts = {}) => request<T>("PATCH", path, body, opts),
  del: <T>(path: string, opts: Opts = {}) => request<T>("DELETE", path, undefined, opts),
};

/** Soft GET: null on any failure, so callers can fall back to fixtures. */
export async function tryGet<T>(path: string, opts: Opts = {}): Promise<T | null> {
  try {
    return await api.get<T>(path, opts);
  } catch {
    return null;
  }
}
