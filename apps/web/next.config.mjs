/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  /**
   * The web app is the edge (BFF): browser traffic reaches the Go monolith
   * through same-origin rewrites, so session cookies, the CSRF double-submit
   * cookie, and SameSite=Lax all behave as first-party. No CORS anywhere.
   */
  async rewrites() {
    const api = (process.env.ALEXANDRIA_API_URL ?? "").replace(/\/$/, "");
    if (!api) return [];
    return [{ source: "/api/v1/:path*", destination: `${api}/v1/:path*` }];
  },
};

export default nextConfig;
