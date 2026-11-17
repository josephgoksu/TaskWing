# Rules for working on the TaskWing desktop app

- **The shell holds no state.** Run logic lives in the engine. The app draws engine events and sends commands (`docs/engine-protocol.md`). The demo engine in `ui/js/engine.js` is the only place that invents run behaviour.
- **One design system.** Colours, type and shapes come from `ui/styles/tokens.css`, shared with the website. Add a token there rather than a new colour in a component.
- **No frameworks or runtime packages.** `ui/` is plain HTML, CSS and ES modules. The only npm package is the Tauri CLI, a dev dependency.
- **No network from the window.** No CDN, no web fonts, no analytics. The CSP in `tauri.conf.json` enforces it.
- **Escape everything from the engine.** Use `esc()` for every engine string that goes into HTML, and check types in the reducer.
- **Light on CPU.** No looping animation when idle. Hover and focus effects only. A timer runs only while a running run is on screen.
- **Pictures over text.** Show state as squares, locks, pills and proofs. Use words only for decisions.
- Before a PR: `npm run check`, `cargo clippy --all-targets`, and a code review at xhigh with fixes.
