/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  env: {
    // Go API base; the web data layer falls back to seeded fixtures when unset/unreachable.
    ALEXANDRIA_API_URL: process.env.ALEXANDRIA_API_URL ?? "",
  },
};

export default nextConfig;
