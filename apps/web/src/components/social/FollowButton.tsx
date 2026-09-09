"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/** Follow/unfollow: a quiet verb. Alexandria does not gamify the graph. */
export function FollowButton({ username, initial }: { username: string; initial: boolean }) {
  const router = useRouter();
  const [following, setFollowing] = useState(initial);
  const [busy, setBusy] = useState(false);

  async function toggle() {
    setBusy(true);
    try {
      const res = await fetch(`/api/v1/users/${username}/follow`, {
        method: following ? "DELETE" : "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
        credentials: "same-origin",
        body: "{}",
      });
      if (res.ok) {
        setFollowing(!following);
        router.refresh();
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" className={following ? "btn-print" : "btn-solid"} disabled={busy} onClick={toggle}>
      {following ? "Following" : "Follow"}
    </button>
  );
}
