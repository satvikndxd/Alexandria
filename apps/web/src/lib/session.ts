import { cookies } from "next/headers";
import { tryGet } from "./api";

export type SessionUser = {
  id: string;
  username: string;
  email: string;
  role: string;
  reputation: number;
  display_name: string;
  bio: string;
  is_private: boolean;
};

export type Me = {
  authenticated: boolean;
  user?: SessionUser;
  csrf_token?: string;
  daily_review_allowance?: number;
  library_counts?: { kind: string; n: string }[];
  unread_notifications?: number;
};

/** The server-side session, resolved by forwarding the request's cookies. */
export async function me(): Promise<Me> {
  const cookie = cookies().get("alexandria_session")?.value;
  const res = await tryGet<Me>("/v1/me", { cookie: cookie ? `alexandria_session=${cookie}` : undefined });
  if (!res || !res.authenticated) return { authenticated: false };
  return res;
}

export function requestCookie(): string | undefined {
  const c = cookies().get("alexandria_session")?.value;
  return c ? `alexandria_session=${c}` : undefined;
}
