"use client";

/**
 * Browser-side WebAuthn ceremony helpers.
 *
 * The server issues the ceremony (challenge + session state); the browser
 * talks to the authenticator; the raw assertion is posted back verbatim with
 * the ceremony id in a header. All base64url buffering lives here so the
 * pages stay readable.
 */

export function b64uToBytes(b64u: string): Uint8Array {
  const pad = b64u.replace(/-/g, "+").replace(/_/g, "/");
  const bin = atob(pad + "=".repeat((4 - (pad.length % 4)) % 4));
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export function bytesToB64u(bytes: ArrayBuffer | Uint8Array): string {
  const u8 = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let bin = "";
  for (let i = 0; i < u8.length; i++) bin += String.fromCharCode(u8[i]);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

async function post(path: string, body: unknown, ceremonyId?: string): Promise<Response> {
  return fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(csrf() ? { "X-CSRF-Token": csrf() as string } : {}),
      ...(ceremonyId ? { "X-Ceremony-ID": ceremonyId } : {}),
    },
    credentials: "same-origin",
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

export async function errorMessage(res: Response): Promise<string> {
  try {
    const j = await res.json();
    return j?.error?.message ?? `Request failed (${res.status})`;
  } catch {
    return `Request failed (${res.status})`;
  }
}

/** Registration: create an account, then mint a passkey for it. */
export async function registerWithPasskey(username: string, email: string): Promise<{ ok: true } | { ok: false; error: string }> {
  const reg = await post("/api/v1/auth/register", { username, email });
  if (!reg.ok) return { ok: false, error: await errorMessage(reg) };
  const regJson = await reg.json();
  const ceremonyId = regJson?.ceremony?.ceremony_id as string;
  const options = regJson?.ceremony?.options;
  if (!ceremonyId || !options) return { ok: false, error: "The server issued no ceremony." };

  if (!window.PublicKeyCredential) {
    return { ok: false, error: "This browser has no passkey support — use the email link instead." };
  }
  const pub = options.publicKey;
  const created = await navigator.credentials.create({
    publicKey: {
      ...pub,
      challenge: b64uToBytes(pub.challenge),
      user: {
        ...pub.user,
        id: b64uToBytes(pub.user.id),
      },
      excludeCredentials: (pub.excludeCredentials ?? []).map((c: any) => ({
        ...c,
        id: b64uToBytes(c.id),
      })),
    },
  });
  if (!created) return { ok: false, error: "The authenticator returned nothing." };
  const cred = created as PublicKeyCredential;
  const att = cred.response as AuthenticatorAttestationResponse;
  const finish = await post(
    "/api/v1/auth/passkey/finish-registration",
    {
      id: cred.id,
      rawId: bytesToB64u(cred.rawId),
      type: cred.type,
      response: {
        clientDataJSON: bytesToB64u(att.clientDataJSON),
        attestationObject: bytesToB64u(att.attestationObject),
        transports: att.getTransports?.() ?? [],
      },
    },
    ceremonyId,
  );
  if (!finish.ok) return { ok: false, error: await errorMessage(finish) };
  return { ok: true };
}

/** Login: assert with a passkey. Empty username = discoverable ceremony. */
export async function loginWithPasskey(username: string): Promise<{ ok: true } | { ok: false; error: string }> {
  const begin = await post("/api/v1/auth/passkey/begin-login", username ? { username } : {});
  if (!begin.ok) return { ok: false, error: await errorMessage(begin) };
  const json = await begin.json();
  const ceremonyId = json?.ceremony?.ceremony_id as string;
  const options = json?.ceremony?.options;
  if (!ceremonyId || !options) return { ok: false, error: "The server issued no ceremony." };
  if (!window.PublicKeyCredential) {
    return { ok: false, error: "This browser has no passkey support — use the email link instead." };
  }
  const pub = options.publicKey;
  const asserted = await navigator.credentials.get({
    publicKey: {
      ...pub,
      challenge: b64uToBytes(pub.challenge),
      allowCredentials: (pub.allowCredentials ?? []).map((c: any) => ({
        ...c,
        id: b64uToBytes(c.id),
      })),
    },
  });
  if (!asserted) return { ok: false, error: "The authenticator returned nothing." };
  const cred = asserted as PublicKeyCredential;
  const ass = cred.response as AuthenticatorAssertionResponse;
  const finish = await post(
    "/api/v1/auth/passkey/finish-login",
    {
      id: cred.id,
      rawId: bytesToB64u(cred.rawId),
      type: cred.type,
      response: {
        clientDataJSON: bytesToB64u(ass.clientDataJSON),
        authenticatorData: bytesToB64u(ass.authenticatorData),
        signature: bytesToB64u(ass.signature),
        userHandle: ass.userHandle ? bytesToB64u(ass.userHandle) : undefined,
      },
    },
    ceremonyId,
  );
  if (!finish.ok) return { ok: false, error: await errorMessage(finish) };
  return { ok: true };
}

export async function requestMagicLink(email: string): Promise<{ ok: true } | { ok: false; error: string }> {
  const res = await post("/api/v1/auth/magic-link", { email });
  if (!res.ok) return { ok: false, error: await errorMessage(res) };
  return { ok: true };
}

export async function verifyMagicLink(token: string): Promise<{ ok: true } | { ok: false; error: string }> {
  const res = await post("/api/v1/auth/magic-link/verify", { token });
  if (!res.ok) return { ok: false, error: await errorMessage(res) };
  return { ok: true };
}
