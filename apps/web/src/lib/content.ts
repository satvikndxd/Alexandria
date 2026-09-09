/**
 * Content bridge: one place that knows both worlds.
 *
 * Pages ask this module for view models. It prefers the live Go API and falls
 * back to the seeded public-domain fixtures when the API is absent (local UI
 * work, CI previews, this demo). Nothing downstream knows which world it got,
 * and nothing downstream ever renders invented data as if it were live:
 * fixture rows carry `isSeed`.
 */

import { tryGet } from "./api";
import {
  works as fixtureWorks,
  reviews as fixtureReviews,
  scholarNotes as fixtureNotes,
  clubs as fixtureClubs,
  getWork as fixtureWork,
  getReviewsFor as fixtureReviewsFor,
  getNotesFor as fixtureNotesFor,
  type Work,
  type Review,
  type ScholarNote,
  type Club,
} from "./data";

export type { Work, Review, ScholarNote, Club };

export type CoverHue = Work["coverHue"];

const HUES: CoverHue[] = ["vermilion", "botanical", "gold", "ink"];
export function hueFor(slug: string): CoverHue {
  let h = 0;
  for (let i = 0; i < slug.length; i++) h = (h * 31 + slug.charCodeAt(i)) >>> 0;
  return HUES[h % HUES.length];
}

/* ———— API row shapes (mirror the Go json tags) ———— */

type ApiWorkRow = {
  id: string;
  slug: string;
  title: string;
  subtitle?: string | null;
  description?: string;
  first_published?: number | null;
  is_public_domain?: boolean;
  rating_sum?: number;
  rating_count?: number;
  gutenberg_id?: number | null;
  edition_count?: number;
};

type ApiAuthorRow = { id: string; name: string; role: string; birth_year?: number | null; death_year?: number | null };
type ApiEditionRow = {
  id: string;
  title: string;
  isbn13?: string | null;
  format: string;
  publisher?: string | null;
  language: string;
  page_count?: number | null;
  gutenberg_id?: number | null;
  cover_key?: string | null;
  cover_license?: string | null;
  cover_attribution?: string | null;
};
type ApiReviewRow = {
  id: string;
  username: string;
  display_name?: string | null;
  rating: number;
  title: string;
  body: string;
  has_spoilers: boolean;
  like_count: number;
  created_at: string;
};
type ApiNoteRow = {
  id: string;
  username: string;
  display_name?: string | null;
  field?: string | null;
  scholar_status?: string | null;
  kind: string;
  chapter_ref: string;
  title: string;
  body: string;
  status: string;
  is_community: boolean;
  citation_count?: number;
};
type ApiClubRow = { slug: string; name: string; description: string; member_count: number; current_read_title?: string | null };
type ApiActivityRow = {
  kind: string;
  username: string;
  display_name?: string | null;
  work_slug: string;
  work_title: string;
  excerpt?: string | null;
  rating?: number | null;
  created_at: string;
};

/* ———— mappers ———— */

function workFromApi(r: ApiWorkRow): Work {
  const count = Number(r.rating_count ?? 0);
  const sum = Number(r.rating_sum ?? 0);
  return {
    slug: r.slug,
    title: r.title,
    author: { id: "", name: "", years: "" }, // filled by detail/author rows
    firstPublished: r.first_published ?? 0,
    description: r.description ?? "",
    isPublicDomain: Boolean(r.is_public_domain),
    subjects: [],
    rating: count ? Math.round((sum / count / 2) * 2) / 2 : 0,
    ratingCount: count,
    gutenbergId: r.gutenberg_id ?? undefined,
    coverHue: hueFor(r.slug),
  };
}

function reviewFromApi(r: ApiReviewRow): Review {
  return {
    id: r.id,
    workSlug: "",
    username: r.username,
    displayName: r.display_name ?? r.username,
    rating: r.rating / 2, // API stores half-stars 1..10
    title: r.title,
    body: r.body,
    hasSpoilers: r.has_spoilers,
    likeCount: r.like_count,
    createdAt: r.created_at,
    isSeed: false,
  };
}

/* ———— queries ———— */

export async function listWorks(cookie?: string, subject?: string): Promise<Work[]> {
  const qs = subject ? `&subject=${encodeURIComponent(subject)}` : "";
  const res = await tryGet<{ works: ApiWorkRow[] }>(`/v1/works?limit=48&sort=rating${qs}`, { cookie });
  if (res?.works?.length) return res.works.map(workFromApi);
  return fixtureWorks;
}

export type WorkDetail = {
  work: Work;
  authors: { name: string; role: string; slug?: string }[];
  editions: ApiEditionRow[];
  ratingDistribution: { rating: number; n: number }[];
  reviewCount: number;
  subjects: string[];
  myLibrary: { shelf_kind: string; progress_bp: number } | null;
  fromApi: boolean;
};

export async function workDetail(slug: string, cookie?: string): Promise<WorkDetail | null> {
  const res = await tryGet<{
    work: ApiWorkRow;
    average_rating: number;
    authors: ApiAuthorRow[];
    editions: ApiEditionRow[];
    subjects: { name: string }[];
    rating_distribution: { rating: number; n: number }[];
    review_count: number;
    my_library?: { shelf_kind: string; progress_bp: number } | null;
  }>(`/v1/works/${slug}`, { cookie });
  if (res?.work) {
    const w = workFromApi(res.work);
    w.rating = res.average_rating;
    w.subjects = (res.subjects ?? []).map((s) => s.name);
    return {
      work: w,
      authors: (res.authors ?? []).map((a) => ({ name: a.name, role: a.role })),
      editions: res.editions ?? [],
      ratingDistribution: res.rating_distribution ?? [],
      reviewCount: res.review_count ?? 0,
      subjects: w.subjects,
      myLibrary: res.my_library ?? null,
      fromApi: true,
    };
  }
  const fw = fixtureWork(slug);
  if (!fw) return null;
  return {
    work: fw,
    authors: [{ name: fw.author.name, role: "author" }],
    editions: [],
    ratingDistribution: [],
    reviewCount: fw.ratingCount,
    subjects: fw.subjects,
    myLibrary: null,
    fromApi: false,
  };
}

export async function listReviews(slug: string, cookie?: string): Promise<Review[]> {
  const res = await tryGet<{ reviews: ApiReviewRow[] }>(`/v1/works/${slug}/reviews`, { cookie });
  if (res?.reviews) return res.reviews.map((r) => ({ ...reviewFromApi(r), workSlug: slug }));
  return fixtureReviewsFor(slug);
}

export type NoteView = ScholarNote & { verified: boolean };

export async function listNotes(slug: string, cookie?: string): Promise<NoteView[]> {
  const res = await tryGet<{ notes: ApiNoteRow[] }>(`/v1/works/${slug}/notes`, { cookie });
  if (res?.notes) {
    return res.notes.map((n) => ({
      id: n.id,
      workSlug: slug,
      scholar: n.display_name ?? n.username,
      field: n.field ?? (n.is_community ? "Community Contributor" : "Scholar"),
      kind: n.kind as ScholarNote["kind"],
      chapterRef: n.chapter_ref,
      title: n.title,
      body: n.body,
      citations: [],
      isSeed: false,
      verified: n.scholar_status === "verified" && !n.is_community,
    }));
  }
  return fixtureNotesFor(slug).map((n) => ({ ...n, verified: true }));
}

export type ClubView = {
  slug: string;
  name: string;
  description: string;
  members: number;
  currentReadTitle?: string;
  channels: { name: string; kind: "text" | "voice" | "video"; gated?: string }[];
  isSeed: boolean;
};

export async function listClubs(cookie?: string): Promise<ClubView[]> {
  const res = await tryGet<{ clubs: ApiClubRow[] }>("/v1/clubs", { cookie });
  if (res?.clubs?.length) {
    return res.clubs.map((c) => ({
      slug: c.slug,
      name: c.name,
      description: c.description,
      members: Number(c.member_count ?? 0),
      currentReadTitle: c.current_read_title ?? undefined,
      channels: [],
      isSeed: false,
    }));
  }
  return fixtureClubs.map((c) => ({
    slug: c.slug,
    name: c.name,
    description: c.description,
    members: c.members,
    currentReadTitle: fixtureWorks.find((w) => w.slug === c.currentRead)?.title,
    channels: c.channels,
    isSeed: true,
  }));
}

export type Activity = {
  kind: "review" | "shelf_add" | "note";
  actor: string;
  workSlug: string;
  workTitle: string;
  excerpt?: string;
  when: string;
  isSeed: boolean;
};

export async function communityActivity(cookie?: string): Promise<Activity[]> {
  const res = await tryGet<{ activity: ApiActivityRow[] }>("/v1/community-feed?limit=6", { cookie });
  if (res?.activity) {
    return res.activity.map((a) => ({
      kind: (a.kind === "shelf_add" ? "shelf_add" : "review") as Activity["kind"],
      actor: a.display_name ?? a.username,
      workSlug: a.work_slug,
      workTitle: a.work_title,
      excerpt: a.excerpt ?? undefined,
      when: a.created_at,
      isSeed: false,
    }));
  }
  // Fixture activity is derived from fixture reviews only — never invented.
  return fixtureReviews.slice(0, 4).map((r) => ({
    kind: "review" as const,
    actor: r.displayName,
    workSlug: r.workSlug,
    workTitle: fixtureWorks.find((w) => w.slug === r.workSlug)?.title ?? r.workSlug,
    excerpt: r.body.slice(0, 140) + "…",
    when: r.createdAt,
    isSeed: true,
  }));
}

export function relativeWhen(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const mins = Math.max(1, Math.round((Date.now() - then) / 60000));
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.round(hrs / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(iso).toLocaleDateString(undefined, { month: "short", year: "numeric" });
}
