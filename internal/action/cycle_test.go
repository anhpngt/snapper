package action

import (
	"testing"
	"time"
)

func TestCycler_CyclesOnSameWindowAndAction(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)

	hwnd := uintptr(0x100)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	got := []float64{}
	cur := Rect{}
	for range 5 {
		f := c.Next(hwnd, LeftHalf, cur, now)
		got = append(got, f)
		applied := Target(work, LeftHalf, f)
		c.Applied(hwnd, LeftHalf, applied, now)
		cur = applied
		now = now.Add(time.Second)
	}

	want := []float64{seq[0], seq[1], seq[2], seq[0], seq[1]}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("step %d: got %v want %v", i, got[i], w)
		}
	}
}

func TestCycler_ResetsOnDifferentWindow(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	f1 := c.Next(0x100, LeftHalf, Rect{}, now)
	c.Applied(0x100, LeftHalf, Target(work, LeftHalf, f1), now)

	f2 := c.Next(0x100, LeftHalf, Target(work, LeftHalf, f1), now)
	if f2 != seq[1] {
		t.Fatalf("expected advance on same window, got %v", f2)
	}
	c.Applied(0x100, LeftHalf, Target(work, LeftHalf, f2), now)

	// Switch to a different window - should reset to first step.
	f3 := c.Next(0x200, LeftHalf, Rect{}, now)
	if f3 != seq[0] {
		t.Errorf("expected reset on window change, got %v", f3)
	}
}

func TestCycler_ResetsOnDifferentAction(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	f := c.Next(0x100, LeftHalf, Rect{}, now)
	c.Applied(0x100, LeftHalf, Target(work, LeftHalf, f), now)

	f = c.Next(0x100, LeftHalf, Target(work, LeftHalf, f), now)
	if f != seq[1] {
		t.Fatalf("expected advance, got %v", f)
	}
	c.Applied(0x100, LeftHalf, Target(work, LeftHalf, f), now)

	// Different action - reset.
	f = c.Next(0x100, RightHalf, Rect{}, now)
	if f != seq[0] {
		t.Errorf("expected reset on action change, got %v", f)
	}
}

func TestCycler_ResetsOnManualResize(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	f := c.Next(0x100, LeftHalf, Rect{}, now)
	applied := Target(work, LeftHalf, f)
	c.Applied(0x100, LeftHalf, applied, now)

	// User dragged the window - current rect no longer matches lastApplied.
	drifted := applied
	drifted.Right += 100
	f = c.Next(0x100, LeftHalf, drifted, now)
	if f != seq[0] {
		t.Errorf("expected reset on manual resize, got %v", f)
	}
}

func TestCycler_ResetsOnTimeout(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	f := c.Next(0x100, LeftHalf, Rect{}, now)
	applied := Target(work, LeftHalf, f)
	c.Applied(0x100, LeftHalf, applied, now)

	now = now.Add(11 * time.Second)
	f = c.Next(0x100, LeftHalf, applied, now)
	if f != seq[0] {
		t.Errorf("expected reset after timeout, got %v", f)
	}
}

func TestCycler_ToleratesSmallDrift(t *testing.T) {
	seq := []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}
	c := NewCycler(seq, 10*time.Second, 4)
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 1000}
	now := time.Unix(0, 0)

	f := c.Next(0x100, LeftHalf, Rect{}, now)
	applied := Target(work, LeftHalf, f)
	c.Applied(0x100, LeftHalf, applied, now)

	// Within tolerance - should still advance.
	drifted := applied
	drifted.Right += 3
	f = c.Next(0x100, LeftHalf, drifted, now)
	if f != seq[1] {
		t.Errorf("expected advance within tolerance, got %v", f)
	}
}

func TestTarget_LeftAndRightHalves(t *testing.T) {
	work := Rect{Left: 0, Top: 0, Right: 1000, Bottom: 800}

	left := Target(work, LeftHalf, 0.5)
	want := Rect{Left: 0, Top: 0, Right: 500, Bottom: 800}
	if left != want {
		t.Errorf("LeftHalf@0.5: got %+v want %+v", left, want)
	}

	frac := 2.0 / 3.0
	right := Target(work, RightHalf, frac)
	w := int32(float64(1000) * frac)
	wantR := Rect{Left: 1000 - w, Top: 0, Right: 1000, Bottom: 800}
	if right != wantR {
		t.Errorf("RightHalf@2/3: got %+v want %+v", right, wantR)
	}
}

func TestTarget_TopAndBottomFractions(t *testing.T) {
	work := Rect{Left: -1080, Top: -100, Right: 0, Bottom: 1820}

	top := Target(work, TopHalf, 0.5)
	wantTop := Rect{Left: -1080, Top: -100, Right: 0, Bottom: 860}
	if top != wantTop {
		t.Errorf("TopHalf@0.5: got %+v want %+v", top, wantTop)
	}

	frac := 2.0 / 3.0
	bottom := Target(work, BottomHalf, frac)
	h := int32(float64(work.Height()) * frac)
	wantBottom := Rect{Left: -1080, Top: work.Bottom - h, Right: 0, Bottom: 1820}
	if bottom != wantBottom {
		t.Errorf("BottomHalf@2/3: got %+v want %+v", bottom, wantBottom)
	}
}
