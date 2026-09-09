import { PageFrame } from "@/components/frame/PageFrame";
import type { RightRailData } from "@/components/frame/Rail";
import { GreetingHero } from "@/components/home/GreetingHero";
import { ContinueReading, type ContinueItem } from "@/components/home/ContinueReading";
import { ForYou } from "@/components/home/ForYou";
import { ActivityStream } from "@/components/home/ActivityStream";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { communityActivity, listClubs, listWorks, listReviews } from "@/lib/content";
import { works as fixtureWorks, demoLibrary } from "@/lib/data";

export const dynamic = "force-dynamic";

/**
 * The Atrium: the page a reader lands on. Composed exactly as the reference
 * sheet — greeting panel, continue reading, "For You" filters, activity —
 * with the reading-life rail on the right. Every number on this page is
 * either the reader's own data or clearly-labelled demonstration seed.
 */
export default async function Home() {
  const cookie = requestCookie();
  const session = await me();

  const [works, activity, clubs] = await Promise.all([
    listWorks(cookie),
    communityActivity(cookie),
    listClubs(cookie),
  ]);

  // Author names: the list endpoint carries works only, so fall back to the
  // fixture authorship map when the API is absent.
  const authors: Record<string, string> = {};
  for (const w of works) {
    authors[w.slug] = w.author.name || fixtureWorks.find((f) => f.slug === w.slug)?.author.name || "";
  }

  const continueItem = await continueReading(cookie, session.authenticated);
  const rightRail = await railData(cookie, session.authenticated, clubs);

  return (
    <PageFrame
      pathname="/"
      epigraph="A room without books is a body without a soul."
      attribution="Cicero"
      rightRail={rightRail}
    >
      <GreetingHero name={session.user?.display_name || session.user?.username || null} />
      <ContinueReading item={continueItem} />
      <ForYou works={works} authors={authors} />
      <ActivityStream items={activity} />
    </PageFrame>
  );
}

/* ———— data shaping ———— */

async function continueReading(cookie: string | undefined, signedIn: boolean): Promise<ContinueItem | null> {
  if (signedIn) {
    const res = await tryGet<{
      currently_reading: {
        work_slug: string;
        work_title: string;
        progress_bp: number;
        format?: string | null;
        primary_author?: string | null;
        started_on?: string | null;
      }[];
    }>("/v1/me/currently-reading", { cookie });
    const row = res?.currently_reading?.[0];
    if (row) {
      const work = fixtureWorks.find((w) => w.slug === row.work_slug) ?? {
        ...fixtureWorks[0],
        slug: row.work_slug,
        title: row.work_title,
      };
      const own = await ownExcerpt(cookie, row.work_slug);
      return {
        work: { ...work, slug: row.work_slug, title: row.work_title },
        author: row.primary_author ?? work.author.name,
        progressBp: row.progress_bp,
        formatLine: [row.format ?? "Edition", row.started_on ? `since ${row.started_on}` : ""].filter(Boolean).join(" · "),
        ownExcerpt: own,
      };
    }
    return null;
  }
  // Signed-out: show nothing as if it were the reader's desk; invite instead.
  if (demoLibrary.reading.length === 0) return null;
  const first = demoLibrary.reading[0];
  const work = fixtureWorks.find((w) => w.slug === first.slug);
  if (!work) return null;
  return null; // the desk is empty until you sign in — honest by design
}

async function ownExcerpt(cookie: string | undefined, slug: string): Promise<string | undefined> {
  const reviews = await listReviews(slug, cookie);
  const mine = reviews[0];
  if (!mine) return undefined;
  const clean = mine.body.replace(/\s+/g, " ").trim();
  return clean.length > 150 ? clean.slice(0, 150) + "…" : clean;
}

async function railData(
  cookie: string | undefined,
  signedIn: boolean,
  clubs: { slug: string; name: string; members: number }[],
): Promise<RightRailData> {
  const circles = clubs.slice(0, 3).map((c) => ({
    slug: c.slug,
    name: c.name,
    members: formatCount(c.members),
  }));
  const quote = {
    text: "Read slowly. Some books are to be tasted, others to be swallowed, and some few to be chewed and digested.",
    by: "Francis Bacon",
  };
  if (!signedIn) {
    return {
      streakDays: 0,
      streakCells: Array.from({ length: 14 }, () => "empty"),
      year: { books: 0, pages: 0, notes: 0 },
      circles,
      quote,
    };
  }
  const stats = await tryGet<{
    year: { books_finished: number; pages_read: number; notes: number };
    streak_days: number;
    streak_cells: string[];
  }>("/v1/me/stats", { cookie });
  return {
    streakDays: stats?.streak_days ?? 0,
    streakCells: (stats?.streak_cells ?? Array.from({ length: 14 }, () => "empty")) as RightRailData["streakCells"],
    year: {
      books: stats?.year?.books_finished ?? 0,
      pages: stats?.year?.pages_read ?? 0,
      notes: stats?.year?.notes ?? 0,
    },
    circles,
    quote,
  };
}

function formatCount(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}
