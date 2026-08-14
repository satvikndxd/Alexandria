import type { ScholarNote } from "@/lib/data";
import { IconQuill } from "./Ornament";

const KIND_LABEL: Record<ScholarNote["kind"], string> = {
  context: "Contextual Note",
  linguistic: "Linguistic Note",
  historical: "Historical Note",
  interpretive: "Interpretive Note",
};

export function ScholarNoteCard({ note }: { note: ScholarNote }) {
  return (
    <article className="border-2 border-botanical bg-parchment-light p-5 shadow-block-green">
      <header className="flex items-start justify-between gap-3">
        <div>
          <p className="engraved-label">
            {KIND_LABEL[note.kind]} · {note.chapterRef}
          </p>
          <h3 className="mt-1 font-display text-2xl text-botanical">{note.title}</h3>
        </div>
        <IconQuill size={22} className="shrink-0 text-botanical" />
      </header>

      <p className="mt-3 text-[0.97rem] leading-relaxed text-ink-soft">{note.body}</p>

      <footer className="mt-4 border-t border-botanical/40 pt-3">
        <p className="text-sm text-ink-faint">
          <span className="smallcaps text-botanical">{note.scholar}</span> — {note.field}
        </p>
        {note.citations.map((c, i) => (
          <p key={i} className="mt-1 text-xs italic text-ink-faint">
            {c}
          </p>
        ))}
      </footer>
    </article>
  );
}
