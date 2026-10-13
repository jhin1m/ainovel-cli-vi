# Web SPA round 1 report

## Gate (all green)
`pnpm -C web install --frozen-lockfile && typecheck && lint && test (27 tests) && build`, then `go build ./...`. `git status`: web/node_modules ignored, only web/dist/.gitkeep tracked (build recreates it via scripts/keep-dist.mjs).

## Versions pinned (exact)
vite 8.3.4, react/react-dom 19.3.0, **typescript 5.9.3 (not 7.0.2)**: typescript-eslint 8.71.2 peer is `typescript <6.1.0`, so TS 7 is incompatible. @tanstack/react-router 1.170.44, router-plugin 1.168.45, react-query 5.104.2, react-virtual 3.14.17 (installed, unused until round 2), tailwindcss + @tailwindcss/vite 4.3.3, @vitejs/plugin-react 6.1.2, lucide-react 1.56.0, react-markdown 10.1.0 (unused until round 2), vitest 5.0.4, @testing-library/react 16.3.3, jest-dom 7.0.1, user-event 14.6.8, jsdom 30.1.2, eslint 10.12.0, typescript-eslint 8.71.2, eslint-plugin-react-hooks 7.1.1.
Fonts: `@fontsource-variable/literata` 5.3.0; **`@fontsource-variable/be-vietnam-pro` does not exist** (Be Vietnam Pro is not variable) -> `@fontsource/be-vietnam-pro` 5.3.0, weights 400/500/600/700 (latin, latin-ext, vietnamese via ranges). Literata uses own `src/styles/fonts.css` @font-face (latin + vietnamese only) pointing at the package woff2.
web/pnpm-workspace.yaml was auto-created by pnpm 12 (minimumReleaseAgeExclude list); keep it, required for frozen install.

## Decisions
- shadcn CLI not used: plain Tailwind components (src/components/ui: button, field, dialog, states). Fewer deps, no network scaffolding.
- Tailwind v4 `@theme inline` maps CSS vars in tokens.css to utilities (bg-surface, text-st-review...). Dark palette is NOT in the spec (only reader-dark); I derived it (bg #101214, surface #181B1F, accent #3FB5A3, status colours lightened). Review it.
- Lint ban on `dangerouslySetInnerHTML`: `no-restricted-syntax` (JSXAttribute + Identifier); verified it fires.
- 401 handling: api/client calls a handler (main.tsx wires it to router.navigate('/login') + queryClient.clear()); skipped for `/api/auth/*` and `/api/me` (route guard handles `/api/me`). Wrong current password is 403 on the server so no false redirects.
- Guard: `_app` layout route beforeLoad -> ensureQueryData(me); /login and /signup redirect to / when already signed in.
- Add-key is a modal dialog (form remounted per open so password/key never linger). Account page has change password + logout; keys page also shows email, link and logout.
- Library filter chips (Tất cả/Đang viết/Chờ duyệt/Hoàn thành) from the design; list polls every 15s.
- `src/routeTree.gen.ts` is committed (needed for typecheck before first build). `types.gen.ts` committed; tygo emits `BookStatus = string`, so `src/api/status.ts` holds the known-status union.
- tygo pinned v0.2.21 in Makefile; module path in tygo.yaml is github.com/voocel/ainovel-cli. `make gen` ran OK without touching go.mod/go.sum.
- Budget max hard-coded to 50 US$ in the form (design says max 50); server limit `MaxBookBudgetUSD` is not exposed to the client.

## Structure
web/src: main.tsx, router.tsx, routeTree.gen.ts; routes/{__root,login,signup,_app,_app/index,_app/books/new,_app/books/$id/index,_app/settings/{keys,account}}.tsx; api/{client,queries,status,types.gen}.ts; lib/{messages,format}.ts; components/{app-shell,status-badge}.tsx + ui/*; features/{auth,books,keys}/*; styles/{tokens,fonts,app}.css; test/render.tsx.
Tests: api/client.test.ts, components/status-badge.test.tsx, features/books/new-book-logic.test.ts, features/books/new-book-form.test.tsx.
Other files: Makefile, tygo.yaml, web/{package.json,vite.config.ts,eslint.config.js,tsconfig.json,index.html,.gitignore,scripts/keep-dist.mjs}.

## Left for round 2
- /books/$id real screen (placeholder only shows title + badge). Needs route `$id/read/$n` (route dir `_app/books/$id/` already exists), use-book-stream hook, timeline (virtual), stream text, steer box, banners, budget dialog, pause/resume/export, outline/chapters tabs, reader + settings, markdown sanitisation tests. Query hooks (chapters, chapter, outline, pause/resume/next/steer, patch) already exist in api/queries.ts.
- Reader should hide the bottom nav (AppShell currently wraps all `_app` routes); consider a layout split.
- Browser e2e screenshots, 360px / dark-mode pass not done (no browser run this round).
- Dev server (`pnpm dev`) not exercised against a live Go server.

## Missing API fields / notes
- None blocking. Nice-to-have: expose max budget (`MaxBookBudgetUSD`) via an endpoint; list endpoint returns `snapshot` per book so cards work. Unverified: GET /api/keys, /api/books, /api/books/{id}/chapters return `[]` (not null) when empty; client tolerates null anyway.

Status: DONE_WITH_CONCERNS
Summary: Scaffold, tokens, types, client/queries, auth, keys/account, library, new-book screens and tests all green; go build passes with embedded dist.
Concerns/Blockers: TS pinned 5.9.3 (typescript-eslint peer); Be Vietnam Pro uses non-variable @fontsource package; dark palette derived by me; not browser-tested.
