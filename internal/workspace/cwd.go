package workspace

import (
	"sync"
	"time"
)

// cwdRefreshInterval bounds how often a pane's working directory is re-read
// from /proc while its shell is producing output.
const cwdRefreshInterval = 500 * time.Millisecond

// cwdRefresher keeps a pane's LastKnownCWD (shown in its title) current. It is
// driven by PTY output, so an idle pane costs nothing: a shell redraws its
// prompt after `cd`, which is what triggers the refresh. Output bursts are
// throttled, with one trailing refresh so the last prompt is never missed.
type cwdRefresher struct {
	pane   *Pane
	redraw func()

	mu      sync.Mutex
	last    time.Time
	timer   *time.Timer
	stopped bool
}

// onOutput is called for every chunk of PTY output, before the redraw signal.
func (c *cwdRefresher) onOutput() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	if wait := cwdRefreshInterval - time.Since(c.last); wait > 0 {
		if c.timer == nil {
			c.timer = time.AfterFunc(wait, c.fire)
		}
		c.mu.Unlock()
		return
	}
	c.last = time.Now()
	c.mu.Unlock()
	c.pane.GetCWD()
}

// output is the redraw hook handed to the PTY pump: refresh, then signal.
func (c *cwdRefresher) output() {
	c.onOutput()
	if c.redraw != nil {
		c.redraw()
	}
}

func (c *cwdRefresher) fire() {
	c.mu.Lock()
	c.timer = nil
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.last = time.Now()
	c.mu.Unlock()

	before := c.pane.cachedCWD()
	if c.pane.GetCWD() != before && c.redraw != nil {
		c.redraw()
	}
}

func (c *cwdRefresher) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}
