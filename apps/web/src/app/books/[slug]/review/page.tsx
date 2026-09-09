import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { ReviewForm } from "@/components/reviews/ReviewForm";
import { me } from "@/lib/session";
import { workDetail } from "@/lib/content";

export const dynamic = "force-dynamic";

export default async function ReviewPage({ params }: { params: { slug: string } }) {
  const session = await me();
  const detail = await workDetail(params.slug);

  return (
    <PageFrame pathname="" epigraph="Judgement is the whole of taste." attribution="Sainte-Beuve">
      <p className="engraved-label">Review</p>
      <h1 className="mt-1 text-[clamp(1.5rem,3vw,2rem)] text-ink">{detail?.work.title ?? params.slug}</h1>
      {!session.authenticated ? (
        <div className="panel mt-6 px-6 py-6">
          <p className="pullquote max-w-[58ch]">
            Reviews are signed, rate-limited, and at least 150 characters long — the platform's whole
            defence against slop. Sign in to add yours.
          </p>
          <Link href="/signin" className="btn-solid mt-4">
            Sign in
          </Link>
        </div>
      ) : (
        <ReviewForm slug={params.slug} title={detail?.work.title ?? params.slug} />
      )}
    </PageFrame>
  );
}
