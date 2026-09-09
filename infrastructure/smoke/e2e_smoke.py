#!/usr/bin/env python3
"""Alexandria end-to-end smoke test (no browser required).

Drives the real stack through the web edge (Next rewrites → Go monolith →
Postgres), exactly as a browser would:

  1. magic-link ceremony (challenge crafted the way the server hashes it)
  2. session cookie + CSRF double-submit token
  3. CSRF enforcement (mutation without the token must be 403)
  4. shelving + reading progress (Row-Level-Security scoped)
  5. SSR pages: home, library, book page, review publish

Requires: psycopg2-binary, a migrated database, `cmd/api` on :8080 and
`next start` on :3000 with ALEXANDRIA_API_URL=http://127.0.0.1:8080 (the
rewrite is baked at build time — build with the env var set).

    python3 infrastructure/smoke/e2e_smoke.py
"""

import base64
import hashlib
import hmac
import http.cookiejar
import json
import sys
import urllib.error
import urllib.request

import psycopg2

BASE = "http://127.0.0.1:3000"
DSN = dict(host="127.0.0.1", port=5433, user="alexandria", dbname="alexandria")
PEPPER = b"alexandria-magiclink-pepper"  # matches SESSION_PEPPER default in dev

FAILURES = []


def check(label, ok, detail=""):
    print(f"{'PASS' if ok else 'FAIL'}  {label}{(' — ' + detail) if detail and not ok else ''}")
    if not ok:
        FAILURES.append(label)


def main():
    jar = http.cookiejar.CookieJar()
    op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def call(method, path, body=None, headers=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(BASE + path, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        for k, v in (headers or {}).items():
            req.add_header(k, v)
        try:
            res = op.open(req, timeout=25)
            return res.status, res.read().decode()
        except urllib.error.HTTPError as e:
            return e.code, e.read().decode()

    # ---- catalogue through the edge ----
    st, body = call("GET", "/api/v1/works?limit=1")
    check("catalogue reachable through web edge", st == 200, body[:120])

    # ---- seed a reader + a work if the instance is empty ----
    conn = psycopg2.connect(**DSN)
    conn.autocommit = True
    cur = conn.cursor()
    cur.execute("SELECT id FROM works WHERE slug='meditations'")
    if not cur.fetchone():
        cur.execute(
            "INSERT INTO works (slug,title,description,is_public_domain,first_published)"
            " VALUES ('meditations','Meditations','Private notes of a Roman emperor.',true,180)"
        )
    cur.execute("DELETE FROM users WHERE username='smokereader'")
    cur.execute("INSERT INTO users (username,email) VALUES ('smokereader','smoke@example.com') RETURNING id")
    uid = cur.fetchone()[0]
    cur.execute("INSERT INTO profiles (user_id) VALUES (%s)", (uid,))
    cur.execute(
        "INSERT INTO shelves (user_id,kind,name) VALUES (%s,'want_to_read','Want to Read'),"
        "(%s,'reading','Currently Reading'),(%s,'read','Read'),(%s,'dnf','DNF'),(%s,'favorites','Favorites')",
        (uid, uid, uid, uid, uid),
    )

    # ---- magic-link ceremony ----
    secret = base64.urlsafe_b64encode(b"T" * 32).decode().rstrip("=")
    mac = hmac.new(PEPPER, base64.urlsafe_b64decode(secret + "=="), hashlib.sha256).digest()
    cur.execute(
        "INSERT INTO auth_challenges (kind,user_id,email,challenge,expires_at)"
        " VALUES ('magic_link',%s,'smoke@example.com',%s, now()+interval '15 minutes') RETURNING id",
        (uid, psycopg2.Binary(mac)),
    )
    challenge = str(cur.fetchone()[0])
    conn.close()

    st, _ = call("POST", "/api/v1/auth/magic-link/verify", {"token": f"{challenge}.{secret}"})
    check("magic-link verify signs in", st == 200)
    st, body = call("GET", "/api/v1/me")
    me = json.loads(body) if st == 200 else {}
    check("session resolves", me.get("authenticated") is True)
    check("csrf token issued", bool(me.get("csrf_token")))
    csrf = me.get("csrf_token", "")

    # ---- CSRF enforcement ----
    st, _ = call("POST", "/api/v1/me/library", {"work_slug": "meditations", "shelf_kind": "reading"})
    check("mutation without csrf is refused", st == 403)

    st, _ = call(
        "POST", "/api/v1/me/library",
        {"work_slug": "meditations", "shelf_kind": "reading"}, {"X-CSRF-Token": csrf},
    )
    check("shelving with csrf succeeds", st == 200)
    st, _ = call(
        "POST", "/api/v1/me/progress",
        {"work_slug": "meditations", "progress_bp": 5600, "format": "public_domain"},
        {"X-CSRF-Token": csrf},
    )
    check("progress update succeeds", st == 200)

    # ---- SSR renders the reader's own state ----
    st, lib = call("GET", "/library")
    check("library SSR shows the shelved book", st == 200 and "Meditations" in lib and "56%" in lib)
    st, home = call("GET", "/")
    check("home SSR shows continue-reading", st == 200 and "Continue Reading" in home)
    st, book = call("GET", "/books/meditations")
    check("book page SSR", st == 200 and "Ratings" in book)

    # ---- friction: a real review passes, a thin one does not ----
    good = (
        "These private notes reward slow reading: the emperor argues with himself across twelve "
        "books, and the repetition is not padding but practice — exercises returned to until they "
        "hold under pressure. I docked half a star only because Book Eleven loses focus."
    )
    st, _ = call(
        "POST", "/api/v1/works/meditations/reviews",
        {"rating": 4.5, "title": "A stoic companion", "body": good, "prompt_why": "It changed my mornings.", "has_spoilers": False},
        {"X-CSRF-Token": csrf},
    )
    check("substantive review accepted", st == 201)
    st, body = call(
        "POST", "/api/v1/works/meditations/reviews",
        {"rating": 5, "body": "great book, loved it"}, {"X-CSRF-Token": csrf},
    )
    check("thin review refused by friction", st == 422, body[:100])

    print()
    if FAILURES:
        print(f"{len(FAILURES)} check(s) failed: {FAILURES}")
        sys.exit(1)
    print("all smoke checks passed")


if __name__ == "__main__":
    main()
