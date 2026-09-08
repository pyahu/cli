# Pyahu CLI website

The product website and Starlight documentation for https://cli.pyahu.io.
Portuguese at `/`, English at `/en/`; documentation at `/docs/` and `/en/docs/`.

```sh
cd website
npm ci
npm run dev
# http://localhost:4321
npm run check
npm run build
```

The design follows Pyahu Platform: deep navy and blue, Space Grotesk and IBM Plex
Mono served locally, shared Pyahu icon geometry, editorial sections and a light
platform section. The terminal is an illustrative session, not a live cluster.

- `src/components/Landing.astro`: product page, keyboard-operable installation
  tabs and clipboard button with success/failure feedback.
- `src/layouts/BaseLayout.astro`: navigation, mobile menu, language links and SEO.
- `src/i18n/landing.ts`: Portuguese and English copy.
- `src/styles/global.css`: responsive marketing design.
- `src/styles/starlight.css`: documentation theme; existing content, search,
  language selection and light/dark modes are preserved.
- `public/install.sh`: existing installer, unchanged by the redesign.

The website build outputs static files into `dist/`. The existing Cloudflare
configuration and publishing flow are retained. `website.yml` runs type checking
and the complete site build for website changes on main and pull requests.

Product scope: Kubernetes local development and validation. Foundation and Cloud
are complementary platform products; AI Factory and Initializer are coming soon.
No claim that local development configuration is production-ready.
