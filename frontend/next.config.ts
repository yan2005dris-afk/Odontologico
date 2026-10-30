/**
 * Next.js configuration for SIGO-Dental Frontend Portal & Chairside PWA (RNF-01, RNF-05, RNF-07, RNF-08).
 */
export interface NextConfig {
  reactStrictMode?: boolean;
  poweredByHeader?: boolean;
  env?: Record<string, string>;
  output?: "standalone" | "export";
}

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  // Emits a self-contained server bundle for the container image (frontend/Dockerfile).
  output: "standalone",
  env: {
    NEXT_PUBLIC_API_GATEWAY_URL: process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "https://api.sigo-dental.local",
    NEXT_PUBLIC_IDLE_TIMEOUT_MS: process.env.NEXT_PUBLIC_IDLE_TIMEOUT_MS ?? "900000",
  },
};

export default nextConfig;
