package app

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"nestedterm/internal/config"
	"nestedterm/internal/keyboard"
	"nestedterm/internal/layout"
	"nestedterm/internal/render"
	"nestedterm/internal/workspace"
)

// App is the top-level application controller.
type App struct {
	cfg         *config.Config
	restorer    *TerminalRestorer
	workspace   *workspace.Workspace
	renderer    *render.Renderer
	decoder     *keyboard.Decoder
	eventCh     chan AppEvent
	stopCh      chan struct{}
	overlays    render.OverlayState
	logger      *log.Logger
	prefixTimer *time.Timer
}

// NewApp creates a new application controller.
func NewApp(cfg *config.Config) *App {
	prefixByte := keyboard.ResolvePrefixByte(cfg.PrefixKey)
	return &App{
		cfg:     cfg,
		decoder: keyboard.NewDecoder(prefixByte),
		eventCh: make(chan AppEvent, 128),
		stopCh:  make(chan struct{}),
		overlays: render.OverlayState{
			HelpText: keyboard.BuildHelpLines(prefixByte),
		},
	}
}

// Run executes the main application lifecycle.
func (a *App) Run() error {
	stdinFd := int(os.Stdin.Fd())

	// 1. Verify interactive terminal
	if !term.IsTerminal(stdinFd) {
		return fmt.Errorf("nestedterm requires an interactive terminal (stdin is not a tty)")
	}

	// 2. Set up debug logging if requested
	if a.cfg.LogFile != "" {
		f, err := os.OpenFile(a.cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			defer f.Close()
			a.logger = log.New(f, "[nestedterm] ", log.LstdFlags|log.Lmicroseconds)
			a.log("Starting nestedterm version %s", config.Version)
		}
	}

	// 3. Read initial terminal dimensions
	cols, rows, err := term.GetSize(stdinFd)
	if err != nil || cols < 4 || rows < 4 {
		return fmt.Errorf("unable to read terminal dimensions: %w", err)
	}

	// 4. Enter raw mode and alternate screen
	restorer, err := EnterRawMode(stdinFd)
	if err != nil {
		return err
	}
	a.restorer = restorer
	defer a.restorer.Restore()

	// 5. Initialize workspace and renderer
	totalRect := layout.Rect{X: 0, Y: 0, W: cols, H: rows}
	onRedraw := func() {
		select {
		case a.eventCh <- AppEvent{Type: EventRedraw}:
		default:
		}
	}

	a.workspace = workspace.NewWorkspace(
		totalRect,
		a.cfg.MaxPanes,
		a.cfg.Shell,
		a.cfg.GraceInterval,
		onRedraw,
	)
	a.renderer = render.NewRenderer(cols, rows, a.cfg.BorderStyle)

	// 6. Spawn initial pane in initial working directory
	initialPane, err := a.workspace.CreateInitialPane(a.cfg.InitialCWD)
	if err != nil {
		return fmt.Errorf("failed to create initial pane: %w", err)
	}
	a.log("Created initial pane: %s in %s", initialPane.ID, a.cfg.InitialCWD)

	// 7. Setup signals: SIGWINCH for resize, termination signals for controlled shutdown
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, unix.SIGWINCH, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	go func() {
		for {
			select {
			case <-a.stopCh:
				return
			case sig := <-sigCh:
				switch sig {
				case unix.SIGWINCH:
					newCols, newRows, szErr := term.GetSize(stdinFd)
					if szErr == nil && newCols > 0 && newRows > 0 {
						a.eventCh <- AppEvent{
							Type: EventResize,
							Cols: newCols,
							Rows: newRows,
						}
					}
				case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP:
					a.log("Received signal %s, initiating shutdown", sig)
					a.eventCh <- AppEvent{Type: EventShutdown}
				}
			}
		}
	}()

	// 8. Start input reader
	StartInputReader(a.decoder, a.eventCh, a.stopCh)

	// 9. Initial paint
	_ = a.renderer.Render(a.workspace, a.overlays, os.Stdout)

	// 10. Main event loop with coalesced rendering
	return a.eventLoop()
}

func (a *App) eventLoop() error {
	ticker := time.NewTicker(25 * time.Millisecond) // ~40 FPS redraw coalesce limit
	defer ticker.Stop()

	dirty := false

	for {
		select {
		case <-a.stopCh:
			return nil

		case <-ticker.C:
			if dirty {
				dirty = false
				_ = a.renderer.Render(a.workspace, a.overlays, os.Stdout)
			}

		case ev := <-a.eventCh:
			switch ev.Type {
			case EventRedraw:
				dirty = true

			case EventResize:
				a.log("Terminal resized to %dx%d", ev.Cols, ev.Rows)
				newRect := layout.Rect{X: 0, Y: 0, W: ev.Cols, H: ev.Rows}
				a.renderer.Resize(ev.Cols, ev.Rows)
				if err := a.workspace.Resize(newRect); err != nil {
					a.overlays.BannerMessage = "Screen too small for layout"
				} else {
					a.overlays.BannerMessage = ""
				}
				dirty = true

			case EventShutdown:
				a.log("Shutdown requested")
				a.shutdown()
				return nil

			case EventPrefixTimeout:
				if a.decoder.Mode == keyboard.ModePrefixPending {
					a.decoder.ResetMode()
					a.overlays.BannerMessage = ""
					dirty = true
				}

			case EventKey:
				for _, k := range ev.KeyEvents {
					shouldExit := a.handleKeyEvent(k)
					if shouldExit {
						a.shutdown()
						return nil
					}
				}
				dirty = true
			}
		}
	}
}

func (a *App) handleKeyEvent(ev keyboard.DecodeEvent) bool {
	activePane := a.workspace.GetActivePane()

	// 1. Modal Help Overlay
	if a.overlays.ShowHelp {
		a.overlays.ShowHelp = false
		a.decoder.ResetMode()
		return false
	}

	// 2. Modal Quit Confirmation
	if a.overlays.ShowQuitConfirm {
		if len(ev.RawBytes) > 0 {
			b := ev.RawBytes[0]
			if b == 'y' || b == 'Y' || b == '\r' || b == '\n' {
				return true // confirm exit
			}
		}
		// Cancel
		a.overlays.ShowQuitConfirm = false
		a.decoder.ResetMode()
		return false
	}

	// 3. Scrollback Mode
	if activePane != nil && activePane.InScrollback {
		if len(ev.RawBytes) > 0 {
			raw := string(ev.RawBytes)
			switch {
			case raw == "k" || raw == "\033[A":
				activePane.ScrollUp(1)
			case raw == "j" || raw == "\033[B":
				activePane.ScrollDown(1)
			case raw == "\033[5~" || ev.RawBytes[0] == 0x02 || ev.RawBytes[0] == 0x15: // PgUp / Ctrl+B / Ctrl+U
				activePane.ScrollUp(activePane.InnerRect.H / 2)
			case raw == "\033[6~" || ev.RawBytes[0] == 0x06 || ev.RawBytes[0] == 0x04: // PgDown / Ctrl+F / Ctrl+D
				activePane.ScrollDown(activePane.InnerRect.H / 2)
			case raw == "q" || ev.RawBytes[0] == 0x1b || raw == "\r" || raw == "\n":
				activePane.ExitScrollback()
				a.decoder.ResetMode()
			}
		}
		return false
	}

	// Clear temporary banner on any key unless entering prefix pending
	if a.overlays.BannerMessage != "" && ev.Action != keyboard.ActionPrefixPending {
		a.overlays.BannerMessage = ""
	}

	// Stop prefix timeout timer on key handling
	if a.prefixTimer != nil && ev.Action != keyboard.ActionPrefixPending {
		a.prefixTimer.Stop()
	}

	// 4. Action Dispatch
	switch ev.Action {
	case keyboard.ActionPrefixPending:
		a.overlays.BannerMessage = fmt.Sprintf("[%s]", keyboard.PrefixDisplayName(a.decoder.PrefixByte))
		if a.prefixTimer != nil {
			a.prefixTimer.Stop()
		}
		a.prefixTimer = time.AfterFunc(1500*time.Millisecond, func() {
			a.eventCh <- AppEvent{Type: EventPrefixTimeout}
		})
		return false

	case keyboard.ActionNewPane:
		p, err := a.workspace.CreatePane()
		if err != nil {
			a.overlays.BannerMessage = fmt.Sprintf("Cannot create pane: %v", err)
		} else {
			a.log("Created pane %s in %s", p.ID, p.LastKnownCWD)
		}

	case keyboard.ActionClosePane:
		empty, err := a.workspace.CloseActivePane()
		if err != nil {
			a.log("Error closing active pane: %v", err)
		}
		if empty {
			return true // No panes remain, exit application
		}

	case keyboard.ActionQuit:
		// Check if live managed sessions remain
		hasLiveSessions := false
		for _, p := range a.workspace.AllPanes() {
			if p.Status == workspace.PaneRunning {
				hasLiveSessions = true
				break
			}
		}
		if hasLiveSessions {
			a.overlays.ShowQuitConfirm = true
			a.decoder.Mode = keyboard.ModeModalQuitConfirm
		} else {
			return true // exit immediately
		}

	case keyboard.ActionFocusLeft:
		a.workspace.FocusDirection(layout.DirectionLeft)
	case keyboard.ActionFocusDown:
		a.workspace.FocusDirection(layout.DirectionDown)
	case keyboard.ActionFocusUp:
		a.workspace.FocusDirection(layout.DirectionUp)
	case keyboard.ActionFocusRight:
		a.workspace.FocusDirection(layout.DirectionRight)

	case keyboard.ActionScrollback:
		if activePane != nil {
			activePane.EnterScrollback()
			a.decoder.Mode = keyboard.ModeScrollback
		}

	case keyboard.ActionHelp:
		a.overlays.ShowHelp = true
		a.decoder.Mode = keyboard.ModeModalHelp

	case keyboard.ActionSendPrefix:
		if activePane != nil && len(ev.RawBytes) > 0 {
			_, _ = activePane.Write(ev.RawBytes)
		}

	case keyboard.ActionNone:
		// Normal keyboard input forwarded to active pane
		if activePane != nil && len(ev.RawBytes) > 0 {
			_, _ = activePane.Write(ev.RawBytes)
		}
	}

	return false
}

func (a *App) shutdown() {
	if a.prefixTimer != nil {
		a.prefixTimer.Stop()
	}
	close(a.stopCh)
	if a.workspace != nil {
		a.workspace.CloseAll()
	}
	if a.restorer != nil {
		a.restorer.Restore()
	}
}

func (a *App) log(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	}
}
