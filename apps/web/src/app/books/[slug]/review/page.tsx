import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getWork } from "@/lib/data";
import { SectionHeading } from "@/components/Ornament";
import { ReviewForm } from "./ReviewForm";

export const metadata: Metadata = { title: "Write a Review" };

export default function ReviewPage({ params }: { params: { slug: string } }) {
  const work = getWork(params.slug);
  if (!work) notFound();

  return (
    <div className="mx-auto max-w-2xl space-y-8">
      <SectionHeading caption={`${work.title} — ${work.author.name}`} title="Write a Review" />
      <ReviewForm workTitle={work.title} workSlug={work.slug} />
    </div>
  );
}
