# NestedTerm Architecture & Dependency Decisions

## 1. Overview

NestedTerm is a Linux terminal multiplexer that transforms an existing terminal window into an automatically tiled workspace of independent, simultaneously visible terminal sessions.

Unlike conventional multiplexers or terminal window managers, NestedTerm:
- Runs directly inside the host terminal emulator (X11/Wayland-independent).
- Does not wrap external multiplexers (`tmux`, `screen`).
- Does not rely on generic TUI frameworks (`bubbletea`, `tview`, `ink`).
- Manages an independent pseudo-terminal (PTY) pair and virtual terminal (VT) state for every pane.
- Uses its own ANSI compositor and layout engine to render all panes concurrently.

---

## 2. Dependency Evaluation and Selection

### 2.1 PTY Management: `github.com/creack/pty` (Pinned: v1.1.24)

`github.com/creack/pty` is the industry-standard Go package for Unix pseudo-terminals.

**Evaluation & Capabilities:**
- Full support for opening PTY master/slave pairs on Linux.
- `StartWithSize(cmd, &pty.Winsize{Rows, Cols})` starts the child process with a controlling terminal and initializes dimensions.
- `Setsize(pty, &pty.Winsize{Rows, Cols})` propagates outer or layout-driven resize events by issuing `TIOCSWINSZ` ioctl to the PTY master, raising `SIGWINCH` in the child session.
- Safe closing of master and slave descriptors on session cleanup.

### 2.2 Terminal Emulation: `tuios.dev/vt` (Pinned: v0.3.0)

We evaluated two candidate terminal emulator libraries:
1. `github.com/charmbracelet/x/vt`
2. `tuios.dev/vt`

**Decision:** `tuios.dev/vt` (v0.3.0) was chosen as the primary terminal emulator engine.

**Justification:**
- **In-Process Virtual Screen Buffer:** Provides high-performance virtual screen modeling in pure Go.
- **Structured Cell Access:** Exposes cell data through `CellAt(x, y) *uv.Cell` (from `github.com/charmbracelet/ultraviolet`), preserving UTF-8 grapheme clusters, display widths, truecolor foreground/background values, and ANSI attributes (bold, dim, underline, reverse, italic).
- **Bidirectional Query Responses:** `tuios.dev/vt.Terminal` natively queues responses for device attributes (DA1, DA2), cursor position reports (CPR), and mode requests, exposing `TakeResponses() []byte` for non-blocking draining. This ensures interactive programs (`vim`, `less`, `htop`, `fzf`) do not block waiting for replies.
- **Scrollback History:** Provides `ScrollbackLen()` and `ScrollbackLine(index)` for efficient scrollback browsing.
- **Alternate Screen Buffer:** Accurately switches between main and alternate screen buffers, preserving main screen contents when full-screen applications exit.
- **Concurrency & Resource Lifecycle:** Offers thread-safe state access and explicit `Close()` semantics.

### 2.3 Outer Terminal Management: `golang.org/x/term` (Pinned: v0.47.0)

Used for raw mode switching (`term.MakeRaw`) and restoring host terminal attributes (`term.Restore`), as well as detecting whether stdin is an interactive terminal (`term.IsTerminal`).

---

## 3. Subsystem Architecture

```text
nestedterm executable
│
├── cmd/nestedterm/
│   └── main.go                  // Entry point, CLI flags, terminal validation
│
├── internal/config/
│   ├── config.go                // Application settings & CLI options
│   └── defaults.go              // Default values (prefix: Ctrl+Space, max panes: 12)
│
├── internal/layout/
│   ├── rectangle.go             // Rect math, containment, coordinate translation
│   └── tiler.go                 // Deterministic recursive/aspect-ratio tiling engine
│
├── internal/terminal/
│   ├── interface.go             // TerminalEmulator abstraction interface
│   └── adapter.go               // Adapter implementation wrapping tuios.dev/vt
│
├── internal/pty/
│   ├── session.go               // PTY master, child exec.Cmd, lifecycle state
│   ├── manager.go               // Session creation, environment & shell resolution
│   ├── io.go                    // Asynchronous PTY I/O & query-response routing
│   ├── cwd_linux.go             // Linux /proc/<pid>/cwd tracking with fallback
│   └── cleanup_linux.go         // Linux session ID discovery & bounded SIGTERM/SIGKILL
│
├── internal/workspace/
│   ├── pane.go                  // Pane state, title, CWD, scrollback offset
│   ├── workspace.go             // Pane registry, active pane, geometry mapping
│   ├── manager.go               // Pane creation, closure, directional focus
│   └── status.go                // Pane lifecycle statuses
│
├── internal/render/
│   ├── cells.go                 // Internal RenderCell definitions
│   ├── canvas.go                // 2D screen canvas & double-buffer representation
│   ├── styles.go                // ANSI/SGR color and attribute styling
│   └── renderer.go              // Outer compositor, borders, titles, cursor mapping
│
├── internal/keyboard/
│   ├── prefix.go                // Prefix state machine (Ctrl+Space)
│   ├── decoder.go               // Byte-stream VT parser & escape sequence decoder
│   └── bindings.go              // Keybindings map and action dispatch
│
└── internal/app/
    ├── app.go                   // Main application controller & event loop
    ├── events.go                // Event bus (PTY output, keys, resize, exit)
    ├── input.go                 // Host stdin reader & key router
    └── shutdown.go              // Clean terminal restoration & session termination
```

---

## 4. Key Behavioral Guarantees

1. **PTY Content Isolation:** Child process escape sequences never reach the outer terminal directly. All output is parsed into virtual cells in memory, and the renderer draws only within the pane's assigned screen rectangle.
2. **Deterministic Tiling:** Layout calculation is a pure function of terminal dimensions and pane count. Adding or closing a pane never restarts shells.
3. **Session Containment:** On Linux, processes spawned by child shells belong to the same session ID. NestedTerm discovers all descendant processes in `/proc` and sends SIGTERM followed by SIGKILL if they do not exit within the grace period (750ms).
4. **Resilience & Restoration:** Terminal raw mode and alternate screen mode are restored unconditionally on application exit, external termination signals (SIGINT, SIGTERM, SIGHUP), or panic recovery.
