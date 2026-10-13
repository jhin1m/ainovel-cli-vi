# Phase 6 PWA report

## Done
- vite-plugin-pwa 2.0.0 (+ peers workbox-build 7.4.1, workbox-window 7.4.1), all pinned exact, devDeps. Vite 8 is in peer range.
- registerType 'prompt', injectRegister false (no inline script, CSP-safe); registration via `virtual:pwa-register` in `web/src/lib/pwa-update.ts`, PROD only.
- Update UX: tiny store + `components/update-toast.tsx` ("Có phiên bản mới" / Cập nhật / Để sau), mounted in `main.tsx`. Strings in `lib/messages.ts` (`msg.update`).
- Manifest: AI Novel, lang vi, start_url /, standalone, theme #0e6e62, bg #f4f5f3, icons 192/512 + maskable 512.
- Workbox: precache js/css/html/woff2/png/svg, navigateFallback index.html, denylist /^\/api\//, /^\/healthz/, no runtimeCaching, cleanupOutdatedCaches.
- Icons: `web/public/icons/` (icon.svg, maskable.svg sources + PNGs rendered locally with rsvg-convert; no network). apple-touch-icon.png 180.
- index.html: theme-color (light/dark), icon, apple-touch-icon, mobile-web-app-capable, apple-mobile-web-app-*.
- package.json: `"packageManager": "pnpm@12.11.2"`; lockfile updated; tsconfig types += vite-plugin-pwa/client.
- Test: `src/lib/pwa-update.test.ts` (5 cases).

## Gate
install --frozen-lockfile, typecheck, lint, test (53 pass), build all green. dist has sw.js, workbox-*.js, manifest.webmanifest, icons/, .gitkeep kept. `go build ./...` ok; `make build` ok; `./ainovel-cli serve --help` exit 0.

## Server headers (internal/server/spa.go, read only)
Already fine: sw.js/manifest/index.html no-cache; manifest Content-Type application/manifest+json; CSP has worker-src/manifest-src 'self'; .js gets text/javascript from ServeContent. No Go change needed. Note: workbox-*.js (hashed, root) gets max-age=3600, acceptable since name is content-hashed.

## Concerns
- `.github/workflows/web.yml` uses pnpm/action-setup@v4 with `version: 12.11.2` AND package.json now has packageManager. Same value so v4 should accept, but safest is dropping `version:` from the workflow (I did not edit it, outside web/**).
- Lockfile diff is large (pnpm 12 reshuffle + workbox tree); versions of existing deps unchanged.
- Not verified on device: Lighthouse installable, iOS/Android install, SSE with active SW (SW never intercepts /api via precache; no runtime route, so SSE passes through) — needs staging test.
- `./ainovel-cli` binary is gitignored.
