"use client";

import { useState } from "react";
import { PortraitMedallion } from "@/components/Engravings";
import { Stars } from "@/components/Stars";
import { relativeWhen } from "@/lib/content";

/**
 * A review as the sheet sets it: medallion, name, stars, title in serif,
 * body in the literary face. Spoiler-tagged bodies arrive from the API
 * already withheld; for seeded fixtures the veil is client-side and the
 * reader taps to reveal.
 */
export function ReviewRow({
  username,
  displayName,
  rating,
  title,
  body,
  hasSpoilers,
  likeCount,
  createdAt,
  withheld,
}: {
  username: string;
  displayName?: string;
  rating: number; // stars, half steps
  title: string;
  body: string;
  hasSpoilers: boolean;
  likeCount: number;
  createdAt: string;
  withheld?: boolean;
}) {
  const [revealed, setRevealed] = useState(!withheld);
  const veiled = hasSpoilers && !revealed;

  return (
    <li className="hairline flex gap-4 py-5 first:border-t-0">
      <span className="medallion h-11 w-11 shrink-0 text-ink" aria-hidden>
        <PortraitMedallion className="h-11 w-11" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="font-semibold text-ink">{displayName || username}</span>
          <Stars rating={rating} size={13} />
          <span className="text-xs text-ink-faint">{relativeWhen(createdAt)}</span>
          {hasSpoilers ? (
            <span className="engraved-label !text-vermilion">Spoilers</span>
          ) : null}
        </div>
        {title ? <h3 className="mt-1 text-[1.05rem] font-semibold text-ink">{title}</h3> : null}
        <p
          className={`pullquote mt-2 whitespace-pre-line !text-[0.98rem] ${veiled ? "spoiler-veil" : ""}`}
          onClick={veiled ? () => setRevealed(true) : undefined}
          role={veiled ? "button" : undefined}
          tabIndex={veiled ? 0 : undefined}
          onKeyDown={veiled ? (e) => e.key === "Enter" && setRevealed(true) : undefined}
          aria-label={veiled ? "Spoiler hidden — activate to reveal" : undefined}
        >
          {veiled ? body || "This review discusses endings. Tap to reveal." : body}
        </p>
        <p className="mt-2 text-xs text-ink-faint">
          {likeCount} reader{likeCount === 1 ? "" : "s"} found this worth keeping
          {veiled ? " · tap the blurred text to reveal" : ""}
        </p>
      </div>
    </li>
  );
}
