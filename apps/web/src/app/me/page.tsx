import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { ReviewRow } from "@/components/reviews/ReviewRow";
import { RuleWithFleuron } from "@/components/Engravings";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

/**
 * The profile, set like a literary journal's contributor page: name, bio,
 * the year's reading in figures, shelves in counts, reviews in full.
 * Follower counts exist but are never the headline.
 */
export default async function MePage() {
  const cookie = requestCookie();
  const session = await me();
  if (!session.authenticated || !session.user) {
    return (
      <PageFrame pathname="/me" epigraph="Know thy reader." attribution="Alexandria">
        <h1 className="section-title !text-[1.7rem]">Your page</h1>
        <p className="pullquote mt-3 max-w-[58ch]">Sign in to see your profile as other readers see it.</p>
        <Link href="/signin" className="btn-solid mt-5">
          Sign in
        </Link>
      </PageFrame>
    );
  }

  const username = session.user.username;
  const [stats, reviews, profile] = await Promise.all([
    tryGet<{
      stats: { books_finished: number; books_dnf: number; books_in_progress: number };
      shelves: { kind: string; n: string }[];
      year: { books_finished: number; pages_read: number; notes: number };
      streak_days: number;
    }>(`/v1/me/stats`, { cookie }),
    tryGet<{ reviews: ReviewApiRow[] }>(`/v1/users/${username}/reviews?limit=24`, { cookie }),
    tryGet<{ user: { display_name?: string; bio?: string; member_since?: string } }>(`/v1/users/${username}`, { cookie }),
  ]);

  return (
    <PageFrame pathname="/me" epigraph="Know thy reader." attribution="Alexandria">
      <p className="engraved-label">Your page</p>
      <h1 className="mt-1 text-[clamp(1.6rem,3vw,2.1rem)] text-ink">
        {profile?.user?.display_name || session.user.display_name || username}
      </h1>
      <p className="mt-1 text-sm text-ink-faint">@{username}</p>
      {(profile?.user?.bio || session.user.bio) ? (
        <p className="pullquote mt-3 max-w-[62ch]">{profile?.user?.bio || session.user.bio}</p>
      ) : null}
      <RuleWithFleuron className="mt-6 max-w-[160px]" />

      <div className="mt-6 flex flex-wrap gap-0 border border-ink/60">
        <Stat label="Finished" value={stats?.stats?.books_finished ?? 0} />
        <Stat label="In progress" value={stats?.stats?.books_in_progress ?? 0} />
        <Stat label="Pages this year" value={stats?.year?.pages_read ?? 0} />
        <Stat label="Streak (days)" value={stats?.streak_days ?? 0} />
      </div>

      {stats?.shelves?.length ? (
        <ul className="mt-5 flex flex-wrap gap-2">
          {stats.shelves.map((s) => (
            <li key={s.kind} className="border border-ink/60 px-3 py-1 text-sm text-ink-soft">
              {s.kind.replace(/_/g, " ")} · <span className="oldstyle">{Number(s.n)}</span>
            </li>
          ))}
        </ul>
      ) : null}

      <h2 className="section-title mt-10">Your reviews</h2>
      {reviews?.reviews?.length ? (
        <ul className="mt-2">
          {reviews.reviews.map((r) => (
            <ReviewRow
              key={r.id}
              username={r.username}
              displayName={r.display_name ?? undefined}
              rating={r.rating / 2}
              title={r.title}
              body={r.body}
              hasSpoilers={r.has_spoilers}
              likeCount={r.like_count}
              createdAt={r.created_at}
            />
          ))}
        </ul>
      ) : (
        <p className="pullquote mt-3">Nothing published yet. Your first review is a small act of patronage.</p>
      )}
    </PageFrame>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="stat-col flex-1 px-4 py-3 text-center">
      <p className="oldstyle text-2xl text-ink">{value}</p>
      <p className="engraved-label mt-1">{label}</p>
    </div>
  );
}

type ReviewApiRow = {
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
