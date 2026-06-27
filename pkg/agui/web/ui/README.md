# AG-UI web UI (CopilotKit)

Source for the CopilotKit chat that `pkg/agui/web` serves. It talks to
the AG-UI endpoints under `/api/agui` (agent list, run/SSE, conversation
threads, thread history).

The build output is committed to `../static` and embedded into the Go
binary via `go:embed`, so **`go build` does not need Node** — you only
run the steps below when changing the UI source.

## Develop

```bash
pnpm install          # or: npm install
pnpm dev              # Vite dev server on :5180, proxies /api → :8080
```

Run a Go server with `web.Serve(":8080", client)` (or
`agui.NewHandler`) alongside `pnpm dev`.

## Build (regenerate the embedded assets)

```bash
pnpm install
pnpm build            # writes ../static/{index.html, assets/*, basic.html}
```

Then rebuild the Go binary to pick up the new embedded assets. Commit
the regenerated `../static` output along with your source changes.

## Notes

- `public/basic.html` is the zero-dependency, no-CDN fallback UI. Vite
  copies it into the build output; the embedded server links to it from
  the CopilotKit page's error fallback and it is reachable at
  `/basic.html`.
- CopilotKit v2's dependency graph does not resolve through public ESM
  CDNs (esm.sh / jsDelivr both fail on its `@a2ui/*` transitive deps),
  which is why this is a real Vite build rather than CDN `<script>`
  tags.
- `vite.config.ts` aliases out CopilotKit's heaviest optional deps to
  keep the embedded build ~1MB instead of ~17MB. None are needed by the
  chat as used here:
  - `streamdown` → `src/streamdown-lite.tsx` (a `react-markdown` shim)
    drops Shiki (per-language syntax grammars) + Mermaid + Cytoscape
    (~14MB of lazy chunks).
  - `katex/dist/katex.min.css` → `src/empty.css` drops ~1MB of KaTeX
    math fonts (the shim doesn't render LaTeX).
  - `@copilotkit/web-inspector` → `src/web-inspector-stub.ts` drops the
    ~850KB dev console, which only loads when `showDevConsole` is true
    (we always pass `false`).
