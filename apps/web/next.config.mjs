/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080',
    NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME ?? 'Verdex',
  },
  experimental: {
    typedRoutes: false,
  },
  eslint: {
    // ESLint is enforced by its own dedicated, separately-visible CI
    // step (`make ts-lint` / the "Lint & Typecheck (TypeScript)" job)
    // rather than here, so a pre-existing lint finding doesn't fail
    // the production build a second time via a different code path.
    // Type errors are not exempted: typescript.ignoreBuildErrors is
    // deliberately left at its default (false).
    ignoreDuringBuilds: true,
  },
};

export default nextConfig;
