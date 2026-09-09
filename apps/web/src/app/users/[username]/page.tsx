import Link from "next/link";
import { notFound } from "next/navigation";
import { PageFrame } from "@/components/frame/PageFrame";
import { ReviewRow } from "@/components/reviews/ReviewRow";
import { FollowButton } from "@/components/social/FollowButton";
import { RuleWithFleuron, PortraitMedallion } from "@/components/Engravings";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

type Profile = {
  user: {
    id: string;
    username: string;
    display_name?: string | null;
    bio?: string | null;
    pronouns?: string | null;
    location?: string | null;
    member_since?: string;
    is_private?: boolean;
  };
  follows?: { following: number; followers: number };
  following?: boolean;
};

/** A public profile: existence and public shelves only when private; never a
 *  leaderboard. Follower counts are present but set small, at the foot. */
export default async function UserPage({ params }: { params: { username: string } }) {
  const cookie = requestCookie();
  const session = await me();
  const profile = await tryGet<Profile>(`/v1/users/${params.username}`, { cookie });
  if (!profile?.user) notFound();

  const u = profile.user;
  const reviews = u.is_private
    ? null
    : (await tryGet<{ reviews: ReviewApiRow[] }>(`/v1/users/${params.username}/reviews?limit=24`, { cookie }))?.reviews ?? [];

  return (
    <PageFrame pathname="" epigraph="A reader is a citizen of many countries." attribution="Alexandria">
      <header className="flex items-start gap-5">
        <span className="medallion h-16 w-16 shrink-0 text-ink" aria-hidden>
          <PortraitMedallion className="h-16 w-16" />
        </span>
        <div className="min-w-0">
          <h1 className="text-[clamp(1.5rem,3vw,2rem)] leading-tight text-ink">{u.display_name || u.username}</h1>
          <p className="mt-0.5 text-sm text-ink-faint">
            @{u.username}
            {u.pronouns ? ` · ${u.pronouns}` : ""}
            {u.location ? ` · ${u.location}` : ""}
          </p>
          {session.authenticated && session.user?.username !== u.username ? (
            <div className="mt-3">
              <FollowButton username={u.username} initial={Boolean(profile.following)} />
            </div>
          ) : null}
        </div>
      </header>

      {u.is_private ? (
        <p className="pullquote mt-6 max-w-[58ch]">
          This reader keeps a private library. Their shelves, notes and reviews are theirs alone —
          the database enforces it, not merely the interface.
        </p>
      ) : (
        <>
          {u.bio ? <p className="pullquote mt-5 max-w-[64ch]">{u.bio}</p> : null}
          <RuleWithFleuron className="mt-6 max-w-[160px]" />
          {profile.follows ? (
            <p className="engraved-label mt-4">
              <span className="oldstyle">{profile.follows.following}</span> following ·{" "}
              <span className="oldstyle">{profile.follows.followers}</span> followers
            </p>
          ) : null}

          <h2 className="section-title mt-8">Reviews</h2>
          {reviews?.length ? (
            <ul className="mt-2">
              {reviews.map((r) => (
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
            <p className="pullquote mt-3">No public reviews yet.</p>
          )}
        </>
      )}
      <p className="engraved-label mt-8">
        <Link href="/discover" className="underline">
          Browse the catalogue
        </Link>
      </p>
    </PageFrame>
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
