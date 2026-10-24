# NestedTerm Keybindings Reference

NestedTerm uses a simple prefix-based keyboard interface inspired by standard terminal multiplexers.

By default, the prefix key is **`Ctrl+Space`** (represented in terminal input as the `NUL` byte `0x00`).

---

## 1. Primary Shortcuts

To trigger an action, press the prefix key (`Ctrl+Space`), release it, and then press the action key:

| Shortcut | Action | Description |
|---|---|---|
| `Ctrl+Space n` | **New Pane** | Spawns a new independent shell pane in the active pane's current directory and rebalances the layout. |
| `Ctrl+Space x` | **Close Pane** | Closes the active pane and terminates its managed process session. |
| `Ctrl+Space h` | **Focus Left** | Moves keyboard focus to the nearest pane on the left. |
| `Ctrl+Space j` | **Focus Down** | Moves keyboard focus to the nearest pane below. |
| `Ctrl+Space k` | **Focus Up** | Moves keyboard focus to the nearest pane above. |
| `Ctrl+Space l` | **Focus Right** | Moves keyboard focus to the nearest pane on the right. |
| `Ctrl+Space [` | **Scrollback Mode** | Enters scrollback history navigation for the active pane. |
| `Ctrl+Space ?` | **Help Overlay** | Displays the on-screen shortcut guide. Press any key to dismiss. |
| `Ctrl+Space q` | **Quit** | Requests application exit. Prompts for confirmation if managed processes are active. |
| `Ctrl+Space Ctrl+Space` | **Literal Prefix** | Sends a literal `Ctrl+Space` (byte `0x00`) to the active shell. |

---

## 2. Scrollback Mode Navigation

When in scrollback mode (entered with `Ctrl+Space [`), the active pane's border displays a `[SCROLL: +<offset>]` indicator:

| Key | Action |
|---|---|
| `Up` or `k` | Scroll up 1 line |
| `Down` or `j` | Scroll down 1 line |
| `PageUp` or `Ctrl+U` or `Ctrl+B` | Scroll up half screen |
| `PageDown` or `Ctrl+D` or `Ctrl+F` | Scroll down half screen |
| `q`, `Esc`, or `Enter` | Exit scrollback mode and return to live terminal view |

---

## 3. Configuring Alternate Prefix Keys

If your terminal or workflow prefers a different prefix, you can configure it via CLI flags:

```bash
# Use Ctrl+B (like tmux)
nestedterm --prefix ctrl-b

# Use Ctrl+A (like GNU Screen)
nestedterm --prefix ctrl-a
```

Supported prefixes:
- `ctrl-space` (default)
- `ctrl-b`
- `ctrl-a`

---

## 4. Normal Input Forwarding

When no prefix is active:
- All keystrokes, ASCII control characters (including `Ctrl+C`, `Ctrl+D`, `Ctrl+Z`, `Tab`, `Enter`), UTF-8 runes, and escape sequences (arrow keys, function keys) are forwarded directly and unmodified to the active pane's PTY.
- If an unknown key is pressed after the prefix key (e.g. `Ctrl+Space` followed by `z`), NestedTerm does not swallow your input; it automatically cancels prefix mode and delivers both the literal prefix and the typed key to the active shell.
