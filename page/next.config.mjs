const dev = process.env.NODE_ENV === "development";

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Production builds are a static export served by the Go binary. In
  // development the Next dev server proxies API calls to a local backend.
  output: dev ? undefined : "export",
  images: { unoptimized: true },
  ...(dev && {
    async rewrites() {
      const backend = process.env.BACKEND_URL || "http://127.0.0.1:8080";
      return [
        { source: "/api/:path*", destination: `${backend}/api/:path*` },
        { source: "/healthz", destination: `${backend}/healthz` },
      ];
    },
  }),
};

export default nextConfig;
