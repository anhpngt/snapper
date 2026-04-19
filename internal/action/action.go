package action

type Action int

const (
	LeftHalf Action = iota
	RightHalf
	Maximize
)

type Rect struct {
	Left, Top, Right, Bottom int32
}

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// Target returns the target visual rect for a half-snap action, sized to
// widthFrac of the work area's width and full height.
func Target(work Rect, a Action, widthFrac float64) Rect {
	w := int32(float64(work.Width()) * widthFrac)
	switch a {
	case LeftHalf:
		return Rect{
			Left:   work.Left,
			Top:    work.Top,
			Right:  work.Left + w,
			Bottom: work.Bottom,
		}
	case RightHalf:
		return Rect{
			Left:   work.Right - w,
			Top:    work.Top,
			Right:  work.Right,
			Bottom: work.Bottom,
		}
	}
	return work
}
