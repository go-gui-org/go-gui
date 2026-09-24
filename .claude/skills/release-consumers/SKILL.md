---
name: release-consumers
description:
  Which sibling repos require a go-gui version bump on release. Use when cutting
  a go-gui release, propagating a new tag to consumers, or deciding whether a
  repo is an upstream or downstream dependency.
---

# go-gui release consumers

- Direct go-gui consumers (require a go-gui bump on release): **go-charts,
  go-edit, go-kite, go-map, go-speedtest, go-term**. Verified against each
  repo's `go.mod` `require` (all `github.com/go-gui-org/*`). go-glyph is
  _upstream_ — go-gui depends on it, not the reverse; never a bump target.
- On release, re-verify the list from `go.mod` files; don't rely on memory. New
  consumers get added without updating this note.
