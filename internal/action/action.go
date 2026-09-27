package action

type Action int

const (
	LeftHalf Action = iota
	RightHalf
	TopHalf
	BottomHalf
)

type Rect struct {
	Left, Top, Right, Bottom int32
}

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// Target returns the target visual rect for a directional snap action, sized
// to sizeFrac along the action's axis and full-sized along the other axis.
func Target(work Rect, a Action, sizeFrac float64) Rect {
	switch a {
	case LeftHalf:
		w := int32(float64(work.Width()) * sizeFrac)
		return Rect{
			Left:   work.Left,
			Top:    work.Top,
			Right:  work.Left + w,
			Bottom: work.Bottom,
		}
	case RightHalf:
		w := int32(float64(work.Width()) * sizeFrac)
		return Rect{
			Left:   work.Right - w,
			Top:    work.Top,
			Right:  work.Right,
			Bottom: work.Bottom,
		}
	case TopHalf:
		h := int32(float64(work.Height()) * sizeFrac)
		return Rect{
			Left:   work.Left,
			Top:    work.Top,
			Right:  work.Right,
			Bottom: work.Top + h,
		}
	case BottomHalf:
		h := int32(float64(work.Height()) * sizeFrac)
		return Rect{
			Left:   work.Left,
			Top:    work.Bottom - h,
			Right:  work.Right,
			Bottom: work.Bottom,
		}
	}
	return work
}
