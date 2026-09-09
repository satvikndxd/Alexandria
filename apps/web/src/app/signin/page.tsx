"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";
import { loginWithPasskey, requestMagicLink } from "@/lib/webauthn";

/**
 * Sign-in: passkey first, magic link as the fallback — the same order the
 * server prefers. No password field exists anywhere in Alexandria.
 */
export default function SignInPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"passkey" | "email">("passkey");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function passkey() {
    setBusy(true);
    setError(null);
    const res = await loginWithPasskey(username.trim());
    if (res.ok) {
      router.push("/");
      router.refresh();
      return;
    }
    setError(res.error);
    setBusy(false);
  }

  async function magic() {
    setBusy(true);
    setError(null);
    const res = await requestMagicLink(email.trim());
    setBusy(false);
    if (!res.ok) {
      setError(res.error);
      return;
    }
    setNotice("If that address has an account, a sign-in link is on its way. It expires in 15 minutes and works once.");
  }

  return (
    <PageFrame pathname="/signin" epigraph="Whoever loves a book loves another." attribution="Anonymous">
      <h1 className="section-title !text-[1.7rem]">Sign in</h1>
      <RuleWithFleuron className="mt-4 max-w-[140px]" />

      <div role="tablist" aria-label="Sign-in method" className="mt-6 flex gap-6 border-b border-ink/40">
        <button role="tab" aria-selected={mode === "passkey"} className="tab-link" onClick={() => setMode("passkey")}>
          Passkey
        </button>
        <button role="tab" aria-selected={mode === "email"} className="tab-link" onClick={() => setMode("email")}>
          Email link
        </button>
      </div>

      {mode === "passkey" ? (
        <form
          className="panel mt-5 max-w-md px-6 py-6"
          onSubmit={(e) => {
            e.preventDefault();
            void passkey();
          }}
        >
          <label className="engraved-label" htmlFor="u">
            Username (leave empty to choose a passkey)
          </label>
          <input id="u" className="field mt-1" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
          <p className="pullquote mt-3">
            Your authenticator proves you are you, and proves it only to this origin — a lookalike
            site cannot harvest a passkey.
          </p>
          <button className="btn-solid mt-4" disabled={busy} type="submit">
            {busy ? "Asking your authenticator…" : "Use a passkey"}
          </button>
        </form>
      ) : (
        <form
          className="panel mt-5 max-w-md px-6 py-6"
          onSubmit={(e) => {
            e.preventDefault();
            void magic();
          }}
        >
          <label className="engraved-label" htmlFor="e">
            Email
          </label>
          <input id="e" type="email" className="field mt-1" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
          <button className="btn-solid mt-4" disabled={busy} type="submit">
            {busy ? "Sealing the envelope…" : "Send a sign-in link"}
          </button>
          {notice ? <p className="pullquote mt-4" role="status">{notice}</p> : null}
        </form>
      )}

      {error ? (
        <p role="alert" className="mt-4 border-l-2 border-vermilion pl-3 text-sm text-vermilion">
          {error}
        </p>
      ) : null}
    </PageFrame>
  );
}
