"use client";

import { useEffect, useRef, useState } from "react";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

type RoomState = "idle" | "joining" | "live" | "error";

/**
 * A voice/video room. The token comes from our own monolith — membership and
 * the spoiler gate are decided there — and only then does the LiveKit client
 * load (dynamic import: readers who never join a room never download an SFU
 * SDK). Recording is never granted by the server; there is no record button
 * because there is no grant to honour one.
 */
export function RoomJoin({ channelId, kind }: { channelId: string; kind: "voice" | "video" }) {
  const [state, setState] = useState<RoomState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [people, setPeople] = useState(0);
  const [muted, setMuted] = useState(false);
  const roomRef = useRef<any>(null);

  useEffect(() => () => { roomRef.current?.disconnect?.(); }, []);

  async function join() {
    setState("joining");
    setError(null);
    try {
      const res = await fetch(`/api/v1/channels/${channelId}/room/token`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
        credentials: "same-origin",
        body: "{}",
      });
      if (!res.ok) {
        const j = await res.json().catch(() => ({}));
        throw new Error(j?.error?.message ?? `The room refused entry (${res.status}).`);
      }
      const { token, url } = await res.json();
      const lk = await import("livekit-client");
      const room = new lk.Room();
      roomRef.current = room;
      room.on(lk.RoomEvent.ParticipantConnected, () => setPeople(room.numParticipants));
      room.on(lk.RoomEvent.ParticipantDisconnected, () => setPeople(room.numParticipants));
      room.on(lk.RoomEvent.Disconnected, () => setState("idle"));
      await room.connect(url, token);
      setPeople(room.numParticipants);
      if (kind === "voice") {
        await room.localParticipant.setMicrophoneEnabled(true);
      } else {
        await room.localParticipant.setCameraEnabled(true);
        await room.localParticipant.setMicrophoneEnabled(true);
      }
      setState("live");
    } catch (e) {
      setState("error");
      setError(e instanceof Error ? e.message : "The room could not be joined.");
    }
  }

  async function leave() {
    await roomRef.current?.disconnect?.();
    roomRef.current = null;
    setState("idle");
  }

  async function toggleMute() {
    if (!roomRef.current) return;
    const next = !muted;
    await roomRef.current.localParticipant.setMicrophoneEnabled(!next);
    setMuted(next);
  }

  return (
    <div className="panel px-6 py-6">
      <p className="engraved-label">{kind === "voice" ? "Voice room" : "Video room"}</p>
      <p className="pullquote mt-2 max-w-[60ch]">
        Rooms are for club members, and gated channels stay closed until your reading passes their
        threshold — endings are not overheard either. Nothing is recorded; the server grants no
        recording capability at all.
      </p>
      <div className="mt-4 flex flex-wrap items-center gap-3">
        {state !== "live" ? (
          <button className="btn-solid" disabled={state === "joining"} onClick={join}>
            {state === "joining" ? "Opening the door…" : "Join room"}
          </button>
        ) : (
          <>
            <button className="btn-print" onClick={toggleMute}>
              {muted ? "Unmute" : "Mute"}
            </button>
            <button className="btn-print !border-vermilion !text-vermilion" onClick={leave}>
              Leave
            </button>
            <span className="oldstyle text-sm text-ink-soft">{people} in the room</span>
          </>
        )}
      </div>
      {error ? <p role="alert" className="mt-3 text-sm text-vermilion">{error}</p> : null}
    </div>
  );
}
