import { RuleWithFleuron, RuinsVignette } from "@/components/Engravings";

/**
 * The greeting panel: the reader's name in large serif, a fleuron break, the
 * house tagline in tracked capitals, and the engraved ruins vignette to the
 * right — the reference sheet's hero, one-to-one.
 */
export function GreetingHero({ name }: { name: string | null }) {
  const hour = new Date().getHours();
  const part = hour < 12 ? "Good morning" : hour < 18 ? "Good afternoon" : "Good evening";
  return (
    <section className="panel-strong flex items-stretch gap-6 px-7 py-7 sm:px-9">
      <div className="min-w-0 flex-1">
        <h1 className="text-[clamp(1.7rem,3.4vw,2.4rem)] leading-tight text-ink">
          {part}
          {name ? `, ${name}` : ""}.
        </h1>
        <RuleWithFleuron className="mt-4 max-w-[120px]" />
        <p className="engraved-label mt-4 text-ink-soft">Another page, a brighter you.</p>
      </div>
      <RuinsVignette className="hidden w-[46%] max-w-[340px] shrink-0 self-center md:block" />
    </section>
  );
}
