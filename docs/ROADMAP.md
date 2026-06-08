# Go-Gui Roadmap

Concrete, milestone-driven initiatives. Each independently shippable.
Milestones map to semver bumps from current v0.25.

---

## Future

Items from the old roadmap not yet implemented, plus new directions.

### Media

- **Embedded video/audio** — native media playback widget. Requires
  platform backends (AVPlayer on macOS, GStreamer or PipeWire on Linux,
  Media Foundation on Windows).

### Autocomplete / suggestion list

Extend `InputCfg` with `Suggestions func(string) []string` (debounced
callback). Renders a floating dropdown below the input, navigable by
arrow keys. Partially covered by Combobox for static option lists;
autocomplete handles dynamic/suggestion scenarios.

### Native dark/light mode sync

Auto-switch theme to follow OS appearance preference. Requires:
- `ThemeAuto` mode in the theme system
- `NativePlatform.OSThemePreference()` on each backend
- macOS: `NSApp.effectiveAppearance`
- Linux: `gsettings get org.gnome.desktop.interface color-scheme`
- Windows: registry `AppsUseLightTheme`

### Charting / graphing

Separate `go-charts` package built on go-gui. All framework prerequisites
are complete (canvas view, retained geometry, text measurement, clipping,
mouse events, gradients, animation, custom shaders).

### Community & adoption

- **Contribution guide**: update `CONTRIBUTING.md` with new Makefile targets
- **Issue templates**: add `.github/ISSUE_TEMPLATE/` forms for bugs and
  feature requests
- **GoReleaser**: evaluate for v0.26+ once Makefile release pipeline is
  stable. Right now the CGo + static SDL2 path needs explicit control;
  GoReleaser adds abstraction when it's no longer needed.

