# TaskWing desktop app

The desktop app for TaskWing: give it one task, approve the exam, and get back one tested pull request.

It's a [Tauri 2](https://tauri.app) app. The window is plain HTML, CSS and JavaScript in `ui/`, with no framework and no runtime npm packages. The look comes from the TaskWing website, and the tokens are in `ui/styles/tokens.css`. The Rust shell in `src-tauri/` runs the TaskWing engine and relays its events. It holds no run state of its own.

## Run it

On a Mac you need the Xcode command line tools, Rust and Node 20 or newer.

```sh
xcode-select --install                                           # once
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh   # once
npm install
npm run dev        # opens the app with live reload of ui/
npm run build      # TaskWing.app and a .dmg in src-tauri/target/release/bundle/
```

The first Rust build takes a few minutes. Later builds take seconds.

On Linux, install the WebKit libraries first (`libwebkit2gtk-4.1-dev libayatana-appindicator3-dev librsvg2-dev libxdo-dev libssl-dev`). On Windows, install the Visual Studio C++ build tools; WebView2 ships with Windows 10 and 11.

## The engine

The app starts `taskwing engine --json`, or the binary named in `TASKWING_ENGINE`. It looks for `taskwing` on the PATH and in the usual install folders (`/opt/homebrew/bin`, `/usr/local/bin`, `~/.cargo/bin`, `~/.local/bin`, `~/go/bin`, `~/bin`), because an app opened from the Finder gets a short PATH. If no engine is found, or it exits before its first event (an older `taskwing` with no `engine` command), the app uses its **demo engine**, which plays believable runs so the app can be tried. The sidebar then says "Demo engine". The protocol is in [`docs/engine-protocol.md`](docs/engine-protocol.md).

You can also open `ui/index.html` through any static server (`python3 -m http.server -d ui`) to work on the UI in a browser. It runs on the demo engine there.

## Menu bar

TaskWing keeps a menu bar icon with New task, Open TaskWing and Quit. Closing the window keeps runs going. The icon shows a count when agents need you.

## Checks

CI runs these on every pull request (`.github/workflows/ci.yml`):

```sh
npm run check                         # JS syntax, no remote URLs in ui/
cd src-tauri
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test
```

## Releases

`.github/workflows/release-app.yml` builds macOS (universal), Linux and Windows. Run it by hand for test builds, or push a tag `app-vX.Y.Z` (matching the version in `package.json`, `tauri.conf.json` and `Cargo.toml`) for a draft release. The Mac build is signed and notarised when the `APPLE_*` secrets are set, and ad hoc signed otherwise.

## Licence

AGPL-3.0-only. Geist and Geist Mono are under the SIL Open Font License (`ui/fonts/OFL.txt`).
