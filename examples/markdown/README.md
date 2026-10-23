# Markdown

> **Framework:** text **Description:** Embedded markdown document rendered with
> the Markdown view. Shows headings, lists, code, and tables.

![Preview](screenshot.png)

<!-- explorer: tags=text category=text run=go -->

---

## Run

```sh
go run ./examples/markdown/
```

## What it demonstrates

Embedded markdown document rendered with the Markdown view. Shows headings,
lists, code, and tables.

See `main.go` for the implementation.

## Mermaid diagrams

Mermaid blocks render as PNG images from a Kroki server. This demo turns
external rendering off with `DisableExternalAPIs: true`, so its diagrams show as
code.

To render diagrams:

1. Call `gui.SetMarkdownExternalAPIsEnabled(true)` once at startup.
2. Remove `DisableExternalAPIs: true` from the `MarkdownCfg`.

The default server is the public `https://kroki.io`. It receives the diagram
source and is sometimes slow or down. To use your own server instead:

1. Save this as `compose.yaml`. Mermaid needs two containers. The gateway on
   port 8000 sends Mermaid work to the `mermaid` container, and
   `KROKI_MERMAID_HOST` tells it where that container is:

   ```yaml
   services:
     core:
       image: yuzutech/kroki
       environment:
         - KROKI_MERMAID_HOST=mermaid
       ports:
         - "8000:8000"
     mermaid:
       image: yuzutech/kroki-mermaid
       expose:
         - "8002"
   ```

2. Run `docker compose up -d`.
3. Set `MermaidFetcher: gui.KrokiMermaidFetcher("http://localhost:8000")` on the
   `MarkdownCfg`.
