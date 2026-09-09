"use client";

import { useEffect, useState } from "react";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

function keyToUint8(base64url: string) {
  const padded = base64url.replace(/-/g, "+").replace(/_/g, "/");
  const bin = atob(padded);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

type State = "unknown" | "unsupported" | "off" | "on" | "error";

/**
 * Push opt-in, per device. The switch asks the browser, stores the
 * subscription server-side, and says nothing when push is not configured —
 * an absent feature is not an error to shout about.
 */
export function PushEnable() {
  const [state, setState] = useState<State>("unknown");
  const [endpoint, setEndpoint] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
      setState("unsupported");
      return;
    }
    setState("off");
  }, []);

  async function enable() {
    setError(null);
    try {
      const cfgRes = await fetch("/api/v1/push/config", { credentials: "same-origin" });
      const cfg = await cfgRes.json();
      if (!cfg.enabled) {
        setError("This instance has no push keys configured.");
        return;
      }
      const reg = await navigator.serviceWorker.register("/sw.js");
      await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: keyToUint8(cfg.public_key),
      });
      const json = sub.toJSON();
      const res = await fetch("/api/v1/push/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
        credentials: "same-origin",
        body: JSON.stringify({ endpoint: json.endpoint, keys: json.keys }),
      });
      if (!res.ok) throw new Error("the server refused the subscription");
      setEndpoint(json.endpoint ?? null);
      setState("on");
    } catch (e) {
      setState("error");
      setError(e instanceof Error ? e.message : "push could not be enabled");
    }
  }

  async function disable() {
    setError(null);
    try {
      const reg = await navigator.serviceWorker.getRegistration();
      const sub = await reg?.pushManager.getSubscription();
      if (sub) {
        await fetch("/api/v1/push/subscribe", {
          method: "DELETE",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
          credentials: "same-origin",
          body: JSON.stringify({ endpoint: sub.endpoint }),
        });
        await sub.unsubscribe();
      }
      setState("off");
      setEndpoint(null);
    } catch (e) {
      setState("error");
      setError(e instanceof Error ? e.message : "push could not be disabled");
    }
  }

  if (state === "unsupported" || state === "unknown") return null;
  return (
    <span className="flex flex-wrap items-center gap-2">
      {state !== "on" ? (
        <button className="btn-print !py-1 !text-[0.78rem]" onClick={enable}>
          Enable push on this device
        </button>
      ) : (
        <button className="btn-print !py-1 !text-[0.78rem]" onClick={disable}>
          Disable push on this device
        </button>
      )}
      {error ? <span className="text-xs text-vermilion">{error}</span> : null}
      {endpoint ? <span className="sr-only">push subscription active</span> : null}
    </span>
  );
}
