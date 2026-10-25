# NestedTerm

**NestedTerm** is a high-performance Linux terminal multiplexer written in Go featuring in-process virtual terminal emulation and automatic tiling.

<img width="1920" height="1080" alt="image" src="https://github.com/user-attachments/assets/101e42ec-93aa-4dc8-9687-d8d57d3191e7" />


It runs directly inside your existing terminal window, transforming it into an automatically tiled workspace composed of multiple simultaneously visible, independently running terminal sessions.

```text
┌──────────────────────────────┬──────────────────────────────┐
│ 1: bash (~/project/backend)  │ 2: bash (~/project/backend)  │
│                              │                              │
│ $ git status                 │ $ go run ./cmd/server        │
│ On branch main               │ Listening on :8080           │
│ Working tree clean           │                              │
├──────────────────────────────┼──────────────────────────────┤
│ 3: bash (~/project/frontend) │ 4: bash (~/project/ai-agent) │
│                              │                              │
│ $ bun run dev                │ > Agent running...           │
│ Server ready on :5173        │ Tests passing: 12/12         │
└──────────────────────────────┴──────────────────────────────┘
```

---

## Key Features

- **Direct In-Terminal Execution:** Runs inside your current terminal (Kitty, Alacritty, GNOME Terminal, xterm, foot, WezTerm) without creating desktop windows or requiring X11/Wayland APIs.
- **Embedded Terminal Emulation:** Built with an embedded pure-Go virtual terminal emulator (`tuios.dev/vt`) backed by `github.com/creack/pty`. No external multiplexer wrappers (`tmux`, `screen`).
- **Custom ANSI Compositor:** Written from scratch with an internal cell grid compositor. Does not rely on Bubble Tea, Ink, tview, or other general-purpose TUI frameworks.
- **Simultaneous Visibility:** Live sessions are always simultaneously visible whenever screen geometry permits. No hidden tabs or forced zoom modes.
- **Automatic Deterministic Tiling:** Automatically calculates balanced splits (aspect-ratio aware) on pane creation, closure, or window resize without restarting shells.
- **Dynamic CWD Tracking:** Inspects `/proc/<pid>/cwd` on Linux so newly spawned panes inherit the active pane's current directory, even if you `cd` around.
- **Isolated I/O & Bidirectional Queries:** Full ANSI/VT terminal capabilities including 24-bit truecolor, styles, alternate-screen, cursor positioning, scrollback, and non-blocking device query reply routing (DA, CPR).
- **Process Session Containment:** Gracefully tears down child process trees and background tasks using Linux session discovery and bounded SIGTERM/SIGKILL reaping.

---

## Critical Reality About the Launching Shell

When you launch NestedTerm from your shell:

```bash
cd ~/project/ryanda
nestedterm
```

The shell that launched NestedTerm waits in the background while NestedTerm takes control of the terminal. The original parent shell's in-memory execution state, unexecuted history, functions, and aliases are **not** migrated into child sessions.

Instead, NestedTerm creates its initial managed shell session (`Pane 1`) inside the launching working directory (`~/project/ryanda`) with an interactive shell. When NestedTerm exits, control returns cleanly to the original parent shell with all terminal modes restored.

---

## Quick Start & Developer Workflow

### 1. Launch NestedTerm

```bash
cd ~/project/my-app
nestedterm
```

NestedTerm opens with a single full-screen pane running your interactive shell.

### 2. Common Workflow

1. **Create Dev Server Pane:** Press `Ctrl+Space` then `n`. A second pane opens side-by-side, inheriting the current directory. Start your dev server:
   ```bash
   bun run dev
   ```
2. **Create Backend API Pane:** Press `Ctrl+Space` then `n`. The screen tiles automatically into balanced regions. Navigate into backend and start the server:
   ```bash
   cd ../backend-api && go run ./cmd/server
   ```
3. **Switch Focus:** Move between panes using directional keys:
   - `Ctrl+Space h` (left)
   - `Ctrl+Space j` (down)
   - `Ctrl+Space k` (up)
   - `Ctrl+Space l` (right)
4. **Inspect Scrollback:** Press `Ctrl+Space` then `[` on any pane. Use `Up`/`Down` or `k`/`j` to inspect history. Press `q` to return to live terminal view.
5. **Select & Copy with the Mouse:** Click a pane to focus it, or drag inside a pane to select text. The selection stays within that pane (it never spills into neighbouring panes) and is copied to the system clipboard on release. The wheel scrolls the pane under the pointer. To use your terminal's own selection instead, hold `Shift` while dragging.
6. **Close a Pane:** Press `Ctrl+Space` then `x`. The pane process is terminated and remaining panes rebalance automatically.
7. **Exit Workspace:** Press `Ctrl+Space` then `q`. If sessions are still active, confirm with `y`. All child sessions terminate cleanly and your host terminal is restored.

---

## Keybindings (Default Prefix: `Ctrl+Space`)

| Shortcut | Action |
|---|---|
| `Ctrl+Space n` | Create a new automatically tiled pane |
| `Ctrl+Space x` | Close active pane (terminates its process session) |
| `Ctrl+Space h` | Focus nearest pane to the left |
| `Ctrl+Space j` | Focus nearest pane below |
| `Ctrl+Space k` | Focus nearest pane above |
| `Ctrl+Space l` | Focus nearest pane to the right |
| `Ctrl+Space [` | Enter scrollback mode (`Up`/`Down` or `k`/`j` to scroll, `q` to exit) |
| `Ctrl+Space ?` | Display shortcut help overlay |
| `Ctrl+Space q` | Request application exit |
| `Ctrl+Space Ctrl+Space` | Send literal `Ctrl+Space` (0x00) to shell |

*Note: You can configure the prefix key using `--prefix ctrl-b` or `--prefix ctrl-a`.*

---

## Command Line Interface

```bash
nestedterm [flags]

Flags:
  --shell <path>       Default interactive shell executable (default: $SHELL or /bin/bash)
  --cwd <path>         Initial working directory (default: current directory)
  --max-panes <n>      Maximum number of simultaneous panes (default: 12)
  --prefix <key>       Prefix key: ctrl-space, ctrl-b, ctrl-a (default: ctrl-space)
  --border <style>     Border style: single, rounded, double, heavy (default: single)
  --log <file>         Path to optional debug log file
  -h, --help           Show help
  -v, --version        Show version
```

---

## Build and Installation

### Quick install (prebuilt binary)

```bash
curl -fsSL https://raw.githubusercontent.com/ryandaaa/nestedterm/main/install.sh | sh
```

Installs the latest release for Linux x86_64 or aarch64 into `~/.local/bin` after verifying its SHA-256 checksum. Options: `NESTEDTERM_VERSION=0.1.0` to pin a version, `PREFIX=/usr/local` to change the location (needs write access), and `sh install.sh --uninstall` to remove it. If you prefer to read the script before running it, download `install.sh` first.

### Install from npm

```bash
npm install -g nestedterm
nestedterm
```

The package bundles prebuilt Linux binaries (x64 and arm64) and needs Node.js 16+ only to launch them. It does not run on other operating systems.

### Prerequisites

- Linux (Debian 12/13 or compatible distribution)
- Go toolchain (Go 1.24+ / Go 1.26)
- Standard development tools (`make`, `gcc` optional)

### Build from Source

```bash
make build
```

This generates the standalone binary `./nestedterm`.

### Run Tests

```bash
# Run unit and integration tests
make test

# Run all tests with Go race detector
make test-race
```

### Install for Current User

```bash
make install
```

Installs to `~/.local/bin/nestedterm` (no root privileges required). Ensure `~/.local/bin` is in your `$PATH`.

---

## Architecture and Documentation

Detailed technical documentation is available in the `docs/` directory:
- [Architecture & Design Decisions](docs/architecture.md)
- [Keybindings Guide](docs/keybindings.md)
- [Troubleshooting & Process Lifecycle](docs/troubleshooting.md)

---

## License

Licensed under the [MIT License](LICENSE).
