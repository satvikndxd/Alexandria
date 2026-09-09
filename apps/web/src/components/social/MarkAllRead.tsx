"use client";

import { useRouter } from "next/navigation";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

export function MarkAllRead() {
  const router = useRouter();
  return (
    <button
      className="btn-print !py-1 !text-[0.78rem]"
      onClick={async () => {
        await fetch("/api/v1/me/notifications/read", {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
          credentials: "same-origin",
          body: "{}",
        });
        router.refresh();
      }}
    >
      Mark all read
    </button>
  );
}
