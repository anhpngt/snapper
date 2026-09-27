package action

import "time"

// Cycler is the state machine that decides which size fraction to apply on each
// hotkey press. It cycles through a fixed sequence when the same action
// is pressed repeatedly on the same window, and resets on any of:
//   - different target window
//   - different action
//   - current rect has drifted from the last applied rect (user moved/resized)
//   - idle timeout exceeded
type Cycler struct {
	sequence        []float64
	resetTimeout    time.Duration
	resizeTolerance int32

	lastHWND    uintptr
	lastAction  Action
	lastApplied Rect
	stepIndex   int
	lastPressAt time.Time
	initialized bool
}

func NewCycler(seq []float64, resetTimeout time.Duration, resizeTolerance int32) *Cycler {
	return &Cycler{
		sequence:        seq,
		resetTimeout:    resetTimeout,
		resizeTolerance: resizeTolerance,
	}
}

// Next returns the size fraction to apply for this press and advances the
// cycle. The caller must call Applied after placing the window, passing the
// rect that was actually set, so the next press can detect user drift.
func (c *Cycler) Next(hwnd uintptr, a Action, currentRect Rect, now time.Time) float64 {
	reset := !c.initialized ||
		hwnd != c.lastHWND ||
		a != c.lastAction ||
		now.Sub(c.lastPressAt) > c.resetTimeout ||
		!rectsClose(currentRect, c.lastApplied, c.resizeTolerance)

	if reset {
		c.stepIndex = 0
	} else {
		c.stepIndex = (c.stepIndex + 1) % len(c.sequence)
	}
	return c.sequence[c.stepIndex]
}

// Applied records the rect the caller actually placed, so subsequent presses
// can tell whether the user has moved the window since.
func (c *Cycler) Applied(hwnd uintptr, a Action, applied Rect, now time.Time) {
	c.initialized = true
	c.lastHWND = hwnd
	c.lastAction = a
	c.lastApplied = applied
	c.lastPressAt = now
}

func rectsClose(a, b Rect, tol int32) bool {
	return abs32(a.Left-b.Left) <= tol &&
		abs32(a.Top-b.Top) <= tol &&
		abs32(a.Right-b.Right) <= tol &&
		abs32(a.Bottom-b.Bottom) <= tol
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
