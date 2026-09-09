"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { PageFrame } from "@/components/frame/PageFrame";
import { verifyMagicLink } from "@/lib/webauthn";

function VerifyInner() {
  const params = useSearchParams();
  const router = useRouter();
  const [state, setState] = useState<"working" | "done" | "error">("working");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const token = params.get("token") ?? "";
    if (!token) {
      setState("error");
      setError("That link carries no token.");
      return;
    }
    verifyMagicLink(token).then((res) => {
      if (res.ok) {
        setState("done");
        router.push("/");
        router.refresh();
      } else {
        setState("error");
        setError(res.error);
      }
    });
  }, [params, router]);

  return (
    <div className="panel mt-6 max-w-md px-6 py-6">
      {state === "working" ? <p className="pullquote">Checking the seal on your link…</p> : null}
      {state === "done" ? <p className="pullquote">Signed in. Welcome back.</p> : null}
      {state === "error" ? (
        <>
          <p role="alert" className="text-sm text-vermilion">
            {error}
          </p>
          <p className="pullquote mt-3">Links expire in 15 minutes and work once — ask for another.</p>
        </>
      ) : null}
    </div>
  );
}

export default function VerifyPage() {
  return (
    <PageFrame pathname="/signin" epigraph="The key turns once." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">One-time link</h1>
      <Suspense fallback={<p className="pullquote mt-6">Reading the token…</p>}>
        <VerifyInner />
      </Suspense>
    </PageFrame>
  );
}
