"use client";

import { useState } from "react";
import type { Review } from "@/lib/data";
import { Stars } from "./Stars";
import { IconLaurel } from "./Ornament";

export function ReviewCard({ review }: { review: Review }) {
  const [revealed, setRevealed] = useState(false);
  const veiled = review.hasSpoilers && !revealed;

  return (
    <article className="manuscript-card p-5">
      <header className="flex items-start justify-between gap-4">
        <div>
          <p className="engraved-label">Reader&rsquo;s Review</p>
          <h3 className="mt-1 font-body text-lg font-semibold text-ink">{review.title}</h3>
          <p className="mt-0.5 text-sm text-ink-faint">
            <span className="smallcaps">{review.displayName}</span>
            <span aria-hidden> · </span>@{review.username}
            <span aria-hidden> · </span>
            <time dateTime={review.createdAt} className="oldstyle">
              {review.createdAt}
            </time>
            {review.isSeed && <span className="ml-2 border border-ink/40 px-1 text-[0.6rem] uppercase tracking-engraved">demo seed</span>}
          </p>
        </div>
        <Stars rating={review.rating} size={14} className="shrink-0 pt-1" />
      </header>

      {review.hasSpoilers && !revealed && (
        <button
          onClick={() => setRevealed(true)}
          className="mt-3 border border-vermilion px-2 py-0.5 text-[0.65rem] uppercase tracking-engraved text-vermilion hover:bg-vermilion hover:text-parchment-light"
        >
          Contains spoilers — tap to unveil
        </button>
      )}

      <p
        className={`mt-3 text-[0.97rem] leading-relaxed text-ink-soft ${veiled ? "spoiler-veil" : "spoiler-veil revealed"}`}
        aria-hidden={veiled}
        onClick={() => veiled && setRevealed(true)}
      >
        {review.body}
      </p>

      <footer className="mt-4 flex items-center gap-2 text-botanical">
        <IconLaurel size={16} />
        <span className="oldstyle text-sm">{review.likeCount} readers found this illuminating</span>
      </footer>
    </article>
  );
}
