# NestedTerm Troubleshooting Guide

## 1. Common Issues and Solutions

### 1.1 "nestedterm requires an interactive terminal (stdin is not a tty)"
- **Cause:** NestedTerm was launched in a non-interactive pipe, cron job, or headless script where stdin is redirected.
- **Solution:** NestedTerm requires a real controlling TTY. Run it directly inside an interactive terminal emulator (e.g. Kitty, Alacritty, GNOME Terminal, xterm, foot, WezTerm).

### 1.2 "Screen too small to accommodate layout" / "Cannot create pane: screen too small"
- **Cause:** The outer terminal window does not have enough columns or rows to divide the screen into additional panes while satisfying minimum usable terminal dimensions (minimum 12 columns, 4 rows per pane).
- **Solution:**
  - Increase your terminal window size (maximize or expand dimensions).
  - Decrease font size.
  - Close unused panes using `Ctrl+Space x`.

### 1.3 `Ctrl+Space` does not trigger the prefix
- **Cause:** Some host terminal emulators or desktop window managers intercept `Ctrl+Space` for input method switching (e.g. IBus, Fcitx) or do not send byte `0x00` (NUL).
- **Solution:**
  - Launch NestedTerm with an alternate prefix key:
    ```bash
    nestedterm --prefix ctrl-b
    ```
    or
    ```bash
    nestedterm --prefix ctrl-a
    ```

### 1.4 Restoring Terminal if Abnormally Terminated
- NestedTerm restores terminal raw mode, alternate screen, and cursor visibility automatically on normal exit, `Ctrl+Space q`, and termination signals (`SIGINT`, `SIGTERM`, `SIGHUP`).
- If killed with `kill -9` (`SIGKILL`), the shell will not have a chance to run exit handlers.
- **Recovery:** Run standard Unix terminal recovery:
  ```bash
  reset
  # or
  stty sane
  ```

### 1.5 Colors look wrong or washed out
- Colors from programs that use the 16/256-color palette (`ls --color`, `git diff`, ...) are passed to your terminal as palette indices, so they follow your terminal's theme.
- Only true 24-bit colors and NestedTerm's own borders use RGB. If your terminal does not advertise truecolor (`COLORTERM=truecolor` or `24bit`, or a `TERM` containing `truecolor`/`-direct`), RGB is approximated with the nearest 256-color index. Over SSH `COLORTERM` is often not forwarded.
- Force a mode with `NESTEDTERM_TRUECOLOR=1` (always 24-bit) or `NESTEDTERM_TRUECOLOR=0` (always 256-color).

### 1.6 A pane disappeared / NestedTerm closed by itself
- When the shell in a pane exits (e.g. `exit` or `Ctrl+D`), that pane is closed and the layout is rebalanced. When the last pane's shell exits, NestedTerm exits too, which closes the host terminal tab if it was launched as the terminal's command.

### 1.7 "Cannot send input: pane is not responding"
- Keyboard input is queued per pane (up to 1024 chunks). If the program in the pane stops reading input and the queue fills up, further input is dropped and this banner is shown instead of freezing the UI.

---

## 2. Working Directory Tracking

NestedTerm inspects the Linux kernel's `/proc/<pid>/cwd` symlink to track the real-time working directory of each pane's active shell or foreground command.

**Known Boundaries:**
- If permissions on `/proc/<pid>` are restricted by restrictive container security profiles (e.g., AppArmor or seccomp blocking ptrace/proc readlink), NestedTerm falls back to the directory cached at shell startup.
- Fast sub-shell directory switches that exit before the next check will not be captured.
- The directory is re-read when the pane produces output (at most every 500ms, plus one trailing refresh), so the title follows `cd` when the shell redraws its prompt. A pane that is completely idle does no polling.

---

## 3. Process Containment and Shutdown

When exiting NestedTerm or closing a pane:
1. NestedTerm looks up all processes belonging to the managed session ID in `/proc/<pid>/stat`.
2. It sends `SIGTERM` to the session group.
3. It waits up to the bounded grace interval (default: 750ms).
4. Any remaining processes still active in that session ID are sent `SIGKILL`.
5. Child processes are reaped via `wait`.

**Boundary with Detached Daemons:**
Processes that explicitly call `setsid()` or double-fork to detach into an independent daemon session (e.g., `nohup cmd &` or system services) intentionally detach from the terminal session. Such explicitly daemonized processes will not be killed by terminal session teardown. All normal foreground programs, shells, pipelines, and background job-control tasks are cleanly terminated.

---

## 4. Enabling Debug Logging

To capture diagnostic information without corrupting the terminal display:

```bash
nestedterm --log /tmp/nestedterm.log
```

You can tail the log from another terminal:

```bash
tail -f /tmp/nestedterm.log
```
