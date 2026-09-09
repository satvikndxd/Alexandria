"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

/**
 * The bell: an addressed-notification count, fetched client-side so the
 * server render stays cache-friendly. No badge theatrics — a number, and a
 * link; and for moderators, the door to the queue.
 */
export function NotifBell() {
  const [unread, setUnread] = useState<number | null>(null);
  const [role, setRole] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/v1/me", { credentials: "same-origin" })
      .then((r) => (r.ok ? r.json() : null))
      .then((j) => {
        if (j?.authenticated) {
          setUnread(Number(j.unread_notifications ?? 0));
          setRole(j.user?.role ?? "reader");
        }
      })
      .catch(() => setUnread(null));
  }, []);

  if (unread === null) return null;
  return (
    <span className="flex items-center gap-3">
      {role === "moderator" || role === "admin" ? (
        <Link href="/moderation" className="engraved-label underline underline-offset-2">
          moderation queue
        </Link>
      ) : null}
      <Link href="/notifications" className="engraved-label underline underline-offset-2">
        alerts{unread > 0 ? ` · ${unread}` : ""}
      </Link>
    </span>
  );
}
