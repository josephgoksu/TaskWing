//! TaskWing desktop shell.
//!
//! The shell holds no run state of its own. It starts the TaskWing engine
//! (`taskwing engine --json`), forwards each line the engine prints to the
//! window as an `engine://event`, and writes the window's commands to the
//! engine's stdin, one JSON object per line. When no engine is installed the
//! window falls back to its built-in demo engine.

use std::ffi::{OsStr, OsString};
use std::io::{self, BufRead, BufReader, Read, Write};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::mpsc::{self, Sender};
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};

use serde_json::Value;
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::{TrayIcon, TrayIconBuilder};
use tauri::{AppHandle, Emitter, Manager, RunEvent, State, WindowEvent};

/// The longest line the shell forwards from the engine (1 MiB).
const MAX_LINE: usize = 1 << 20;

/// How long the engine gets to exit after its stdin closes before it is killed.
const STOP_GRACE: Duration = Duration::from_secs(2);

#[derive(Default)]
struct Engine {
    child: Mutex<Option<Child>>,
    /// Lines for the engine's stdin, tagged with the engine's generation. A
    /// writer thread owns the pipe, so a stalled engine never blocks the app.
    tx: Mutex<Option<(u64, Sender<Vec<u8>>)>>,
    generation: AtomicU64,
}

impl Engine {
    /// Closes the engine's stdin, gives it a moment to clean up, then kills it.
    fn stop(&self) {
        if let Ok(mut tx) = self.tx.lock() {
            tx.take();
        }
        if let Ok(mut child) = self.child.lock() {
            if let Some(mut c) = child.take() {
                let deadline = Instant::now() + STOP_GRACE;
                while matches!(c.try_wait(), Ok(None)) && Instant::now() < deadline {
                    thread::sleep(Duration::from_millis(25));
                }
                let _ = c.kill();
                let _ = c.wait();
            }
        }
    }
}

struct Tray(Mutex<Option<TrayIcon>>);

/// The PATH the engine is looked up in and runs with: the app's PATH plus the
/// usual install folders. A Mac app opened from the Finder or the Dock only
/// gets /usr/bin:/bin:/usr/sbin:/sbin, which has none of them.
fn engine_search_path() -> OsString {
    let mut dirs: Vec<PathBuf> = std::env::var_os("PATH")
        .map(|p| std::env::split_paths(&p).collect())
        .unwrap_or_default();
    let mut extra = vec![
        PathBuf::from("/opt/homebrew/bin"),
        PathBuf::from("/usr/local/bin"),
    ];
    if let Some(home) = std::env::var_os("HOME").map(PathBuf::from) {
        for d in [".cargo/bin", ".local/bin", "go/bin", "bin"] {
            extra.push(home.join(d));
        }
    }
    for d in extra {
        if !dirs.contains(&d) {
            dirs.push(d);
        }
    }
    std::env::join_paths(dirs)
        .ok()
        .or_else(|| std::env::var_os("PATH"))
        .unwrap_or_default()
}

/// The engine binary: `TASKWING_ENGINE` if set, else `taskwing` found on `search`.
fn engine_path(search: &OsStr) -> PathBuf {
    if let Some(p) = std::env::var("TASKWING_ENGINE")
        .ok()
        .filter(|p| !p.trim().is_empty())
    {
        return PathBuf::from(p);
    }
    let name = if cfg!(windows) {
        "taskwing.exe"
    } else {
        "taskwing"
    };
    std::env::split_paths(search)
        .map(|d| d.join(name))
        .find(|p| p.is_file())
        .unwrap_or_else(|| PathBuf::from(name))
}

/// Reads one line of at most `MAX_LINE` bytes into `buf` without ever holding
/// more than that in memory. Returns `None` at the end of the stream and
/// `Some(false)` for a line that was too long and has been skipped.
fn read_line_capped(reader: &mut impl BufRead, buf: &mut Vec<u8>) -> io::Result<Option<bool>> {
    buf.clear();
    let n = Read::take(&mut *reader, MAX_LINE as u64 + 1).read_until(b'\n', buf)?;
    if n == 0 {
        return Ok(None);
    }
    if buf.last() == Some(&b'\n') || buf.len() <= MAX_LINE {
        return Ok(Some(true));
    }
    buf.clear();
    reader.skip_until(b'\n')?;
    Ok(Some(false))
}

/// Starts the engine once. Returns "running" when it is up, or an error the
/// window reads as "use the demo engine".
#[tauri::command]
fn engine_start(app: AppHandle, engine: State<'_, Engine>) -> Result<String, String> {
    let mut slot = engine.child.lock().map_err(|_| "engine lock poisoned")?;
    if let Some(c) = slot.as_mut() {
        let writable = engine.tx.lock().map(|tx| tx.is_some()).unwrap_or(false);
        if writable && matches!(c.try_wait(), Ok(None)) {
            return Ok("running".into());
        }
        // Gone, or no longer taking commands: stop it and start a fresh one.
        let _ = c.kill();
        let _ = c.wait();
        slot.take();
    }

    let search = engine_search_path();
    let mut cmd = Command::new(engine_path(&search));
    cmd.args(["engine", "--json"])
        .env("PATH", &search)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null());
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        // CREATE_NO_WINDOW: no console window for the engine.
        cmd.creation_flags(0x0800_0000);
    }
    let mut child = cmd.spawn().map_err(|e| format!("not-found: {e}"))?;

    let stdout = child.stdout.take().ok_or("engine has no stdout")?;
    let mut stdin = child.stdin.take().ok_or("engine has no stdin")?;
    let generation = engine.generation.fetch_add(1, Ordering::SeqCst) + 1;

    let (tx, rx) = mpsc::channel::<Vec<u8>>();
    thread::spawn(move || {
        for line in rx {
            if stdin.write_all(&line).and_then(|_| stdin.flush()).is_err() {
                break;
            }
        }
    });
    *engine.tx.lock().map_err(|_| "engine lock poisoned")? = Some((generation, tx));
    *slot = Some(child);
    drop(slot);

    let handle = app.clone();
    thread::spawn(move || {
        let mut reader = BufReader::new(stdout);
        let mut buf = Vec::new();
        // Whether the engine ever sent a protocol event. The window asks on
        // exit: an engine that never spoke is too old, so the demo takes over.
        let mut spoke = false;
        while let Ok(Some(fits)) = read_line_capped(&mut reader, &mut buf) {
            // Forward only JSON objects with a string "type".
            if !fits {
                continue;
            }
            if let Ok(v) = serde_json::from_slice::<Value>(&buf) {
                if v.get("type").and_then(Value::as_str).is_some() {
                    spoke = true;
                    let _ = handle.emit_to("main", "engine://event", v);
                }
            }
        }
        // Report the exit only if this is still the current engine.
        let current = handle.try_state::<Engine>().is_some_and(|state| {
            state.tx.lock().is_ok_and(|mut tx| {
                let mine = tx.as_ref().is_some_and(|(g, _)| *g == generation);
                if mine {
                    tx.take();
                }
                mine
            })
        });
        if current {
            let _ = handle.emit_to(
                "main",
                "engine://event",
                serde_json::json!({ "type": "engine.exit", "spoke": spoke }),
            );
        }
    });

    Ok("running".into())
}

/// Sends one command object to the engine as a JSON line. It never blocks:
/// the engine's writer thread delivers it.
#[tauri::command]
fn engine_send(cmd: Value, engine: State<'_, Engine>) -> Result<(), String> {
    if cmd.get("type").and_then(Value::as_str).is_none() {
        return Err("a command needs a string \"type\"".into());
    }
    let mut line = serde_json::to_vec(&cmd).map_err(|e| e.to_string())?;
    line.push(b'\n');
    let guard = engine.tx.lock().map_err(|_| "engine lock poisoned")?;
    let (_, tx) = guard.as_ref().ok_or("engine is not running")?;
    // On failure keep `tx`: the reader thread uses it to know this engine is
    // still the current one, and must still report its exit.
    if tx.send(line).is_err() {
        return Err("engine write failed".into());
    }
    Ok(())
}

/// Opens an https link in the default browser. The window can't open new
/// windows itself, and must not navigate away from the app.
#[tauri::command]
fn open_url(url: String) -> Result<(), String> {
    if !url.starts_with("https://")
        || url.len() > 2048
        || url.chars().any(|c| c.is_whitespace() || c.is_control())
    {
        return Err("only https links can be opened".into());
    }
    #[cfg(target_os = "macos")]
    let mut cmd = Command::new("open");
    #[cfg(windows)]
    let mut cmd = {
        let mut c = Command::new("rundll32");
        c.arg("url.dll,FileProtocolHandler");
        c
    };
    #[cfg(all(unix, not(target_os = "macos")))]
    let mut cmd = Command::new("xdg-open");
    let mut child = cmd
        .arg(&url)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|e| e.to_string())?;
    thread::spawn(move || {
        let _ = child.wait();
    });
    Ok(())
}

/// Shows a short count next to the menu bar icon ("" hides it) and sets the tooltip.
#[tauri::command]
fn tray_status(title: String, tooltip: String, tray: State<'_, Tray>) -> Result<(), String> {
    let guard = tray.0.lock().map_err(|_| "tray lock poisoned")?;
    if let Some(icon) = guard.as_ref() {
        let title: String = title.chars().take(8).collect();
        let tooltip: String = tooltip.chars().take(120).collect();
        icon.set_title(if title.is_empty() { None } else { Some(title) })
            .map_err(|e| e.to_string())?;
        icon.set_tooltip(Some(if tooltip.is_empty() {
            "TaskWing".to_string()
        } else {
            tooltip
        }))
        .map_err(|e| e.to_string())?;
    }
    Ok(())
}

fn show_main(app: &AppHandle) {
    if let Some(w) = app.get_webview_window("main") {
        let _ = w.unminimize();
        let _ = w.show();
        let _ = w.set_focus();
    }
}

fn build_tray(app: &AppHandle) -> tauri::Result<TrayIcon> {
    let new_task = MenuItem::with_id(app, "new", "New task", true, None::<&str>)?;
    let open = MenuItem::with_id(app, "open", "Open TaskWing", true, None::<&str>)?;
    let sep = PredefinedMenuItem::separator(app)?;
    let quit = MenuItem::with_id(app, "quit", "Quit TaskWing", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&new_task, &open, &sep, &quit])?;

    let icon = tauri::image::Image::from_bytes(include_bytes!("../icons/tray.png"))?;
    TrayIconBuilder::with_id("taskwing")
        .icon(icon)
        .icon_as_template(true)
        .tooltip("TaskWing")
        .menu(&menu)
        .show_menu_on_left_click(true)
        .on_menu_event(|app, event| match event.id().as_ref() {
            "new" => {
                show_main(app);
                let _ = app.emit_to("main", "tray://new", ());
            }
            "open" => show_main(app),
            "quit" => app.exit(0),
            _ => {}
        })
        .build(app)
}

pub fn run() {
    let app = tauri::Builder::default()
        .manage(Engine::default())
        .manage(Tray(Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![
            engine_start,
            engine_send,
            open_url,
            tray_status
        ])
        .setup(|app| {
            let tray = build_tray(app.handle())?;
            if let Ok(mut slot) = app.state::<Tray>().0.lock() {
                *slot = Some(tray);
            }
            Ok(())
        })
        .on_window_event(|window, event| {
            // Closing the window keeps TaskWing in the menu bar; runs go on.
            if let WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building TaskWing");

    app.run(|handle, event| match event {
        RunEvent::Exit => handle.state::<Engine>().stop(),
        #[cfg(target_os = "macos")]
        RunEvent::Reopen { .. } => show_main(handle),
        _ => {}
    });
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    fn lines(input: Vec<u8>) -> Vec<Option<Vec<u8>>> {
        let mut reader = BufReader::with_capacity(64, Cursor::new(input));
        let mut buf = Vec::new();
        let mut out = Vec::new();
        while let Some(fits) = read_line_capped(&mut reader, &mut buf).unwrap() {
            assert!(buf.capacity() <= 2 * (MAX_LINE + 1));
            out.push(fits.then(|| buf.clone()));
        }
        out
    }

    #[test]
    fn skips_long_lines_and_keeps_the_rest() {
        let mut input = b"{\"type\":\"a\"}\n".to_vec();
        input.extend(std::iter::repeat_n(b'x', MAX_LINE + 10));
        input.extend(b"\n{\"type\":\"b\"}");
        let got = lines(input);
        assert_eq!(got.len(), 3);
        assert_eq!(got[0].as_deref(), Some(&b"{\"type\":\"a\"}\n"[..]));
        assert_eq!(got[1], None);
        assert_eq!(got[2].as_deref(), Some(&b"{\"type\":\"b\"}"[..]));
    }

    #[test]
    fn keeps_a_line_of_exactly_the_limit() {
        let mut input = vec![b'x'; MAX_LINE];
        input.push(b'\n');
        let got = lines(input);
        assert_eq!(got.len(), 1);
        assert_eq!(got[0].as_ref().map(Vec::len), Some(MAX_LINE + 1));
    }
}
