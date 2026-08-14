"use client";

/**
 * ReviewForm — intentional friction, visible and explained.
 *
 * The same rules the Go domain layer enforces (150-char minimum, half-star
 * rating, "why did you rate it this way?" prompt, daily caps) are mirrored
 * here so the reader sees honest, immediate feedback rather than a server
 * rejection. Friction is a feature: it filters slop without pretending to
 * detect it.
 */

import { useMemo, useState } from "react";
import { Stars } from "@/components/Stars";

const MIN_CHARS = 150;

export function ReviewForm({ workTitle, workSlug }: { workTitle: string; workSlug: string }) {
  const [rating, setRating] = useState(0);
  const [body, setBody] = useState("");
  const [why, setWhy] = useState("");
  const [title, setTitle] = useState("");
  const [spoilers, setSpoilers] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const chars = body.trim().length;
  const remaining = Math.max(0, MIN_CHARS - chars);
  const ready = rating > 0 && remaining === 0 && why.trim().length > 0;

  const meter = useMemo(() => Math.min(1, chars / MIN_CHARS), [chars]);

  if (submitted) {
    return (
      <div className="border-2 border-botanical bg-parchment-light p-6 shadow-block-green">
        <p className="engraved-label">Received</p>
        <p className="mt-2 text-[0.97rem] leading-relaxed text-ink-soft">
          Thank you. In the full deployment this review is submitted to{" "}
          <code className="text-sm">POST /v1/works/{workSlug}/reviews</code>, where the Go domain
          layer re-validates it and the daily posting cap is checked against your reputation.
        </p>
      </div>
    );
  }

  return (
    <form
      className="space-y-6"
      onSubmit={(e) => {
        e.preventDefault();
        if (ready) setSubmitted(true);
      }}
    >
      {/* Rating */}
      <fieldset>
        <legend className="engraved-label">Your rating — half stars allowed</legend>
        <div className="mt-2 flex items-center gap-3">
          <input
            type="range"
            min={0.5}
            max={5}
            step={0.5}
            value={rating || 0.5}
            onChange={(e) => setRating(Number(e.target.value))}
            aria-label="Rating in stars"
            className="w-48 accent-vermilion"
          />
          {rating > 0 ? (
            <span className="flex items-center gap-2">
              <Stars rating={rating} size={18} />
              <span className="oldstyle text-sm text-ink-faint">{rating}</span>
            </span>
          ) : (
            <span className="text-sm italic text-ink-faint">slide to rate</span>
          )}
        </div>
      </fieldset>

      {/* Title */}
      <label className="block">
        <span className="engraved-label">Title (optional)</span>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          placeholder={`On ${workTitle}`}
          className="mt-2 w-full border-2 border-ink bg-parchment-light px-3 py-2 font-body text-ink placeholder:italic placeholder:text-ink-faint focus:outline-none focus:ring-0 focus-visible:border-vermilion"
        />
      </label>

      {/* Body with the friction meter */}
      <label className="block">
        <span className="engraved-label">Your review — minimum {MIN_CHARS} characters</span>
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={8}
          placeholder="What did the book do, and how did it do it to you?"
          className="mt-2 w-full border-2 border-ink bg-parchment-light px-3 py-2 font-body leading-relaxed text-ink placeholder:italic placeholder:text-ink-faint focus:outline-none focus-visible:border-vermilion"
        />
        <div className="mt-2 flex items-center gap-3">
          <div className="h-2 flex-1 border border-ink bg-parchment">
            <div
              className={`h-full ${remaining === 0 ? "bg-botanical" : "bg-vermilion"}`}
              style={{ width: `${meter * 100}%` }}
            />
          </div>
          <span className="oldstyle text-xs text-ink-faint" aria-live="polite">
            {remaining > 0 ? `${remaining} characters to go` : `${chars} characters — enough ink`}
          </span>
        </div>
      </label>

      {/* The why prompt */}
      <label className="block">
        <span className="engraved-label">Why did you rate it this way? (required)</span>
        <input
          value={why}
          onChange={(e) => setWhy(e.target.value)}
          placeholder="One honest sentence."
          className="mt-2 w-full border-2 border-ink bg-parchment-light px-3 py-2 font-body text-ink placeholder:italic placeholder:text-ink-faint focus:outline-none focus-visible:border-vermilion"
        />
      </label>

      {/* Spoilers */}
      <label className="flex items-center gap-3">
        <input
          type="checkbox"
          checked={spoilers}
          onChange={(e) => setSpoilers(e.target.checked)}
          className="h-4 w-4 accent-vermilion"
        />
        <span className="text-sm text-ink-soft">
          This review contains spoilers <span className="italic text-ink-faint">(it will be veiled until tapped)</span>
        </span>
      </label>

      <div className="flex items-center gap-4">
        <button
          type="submit"
          disabled={!ready}
          className="border-2 border-ink bg-ink px-6 py-2.5 text-sm uppercase tracking-engraved text-parchment-light shadow-block-vermilion enabled:hover:bg-botanical disabled:cursor-not-allowed disabled:opacity-40"
        >
          Submit for the record
        </button>
        <p className="text-xs italic text-ink-faint">
          New accounts may post 2 reviews per day. Trusted Readers unlock more.
        </p>
      </div>
    </form>
  );
}
