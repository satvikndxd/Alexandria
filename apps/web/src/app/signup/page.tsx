"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";
import { registerWithPasskey } from "@/lib/webauthn";

/** Registration: the account and its first passkey are one ceremony. */
export default function SignUpPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const res = await registerWithPasskey(username.trim(), email.trim());
    if (res.ok) {
      router.push("/");
      router.refresh();
      return;
    }
    setError(res.error);
    setBusy(false);
  }

  return (
    <PageFrame pathname="/signup" epigraph="Begin anywhere." attribution="John Cage">
      <h1 className="section-title !text-[1.7rem]">Take a shelf</h1>
      <RuleWithFleuron className="mt-4 max-w-[140px]" />
      <form className="panel mt-6 max-w-md px-6 py-6" onSubmit={submit}>
        <label className="engraved-label" htmlFor="u">
          Username — 3–32 characters, letters digits . - _
        </label>
        <input id="u" className="field mt-1" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        <label className="engraved-label mt-4 block" htmlFor="e">
          Email — used only for sign-in links and moderation notices
        </label>
        <input id="e" type="email" className="field mt-1" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" required />
        <p className="pullquote mt-4">
          No password is created or stored. Your first passkey is minted during registration; email
          links remain as a fallback forever.
        </p>
        <button className="btn-solid mt-4" disabled={busy} type="submit">
          {busy ? "Minting your passkey…" : "Create account & add passkey"}
        </button>
        {error ? (
          <p role="alert" className="mt-4 border-l-2 border-vermilion pl-3 text-sm text-vermilion">
            {error}
          </p>
        ) : null}
      </form>
    </PageFrame>
  );
}
