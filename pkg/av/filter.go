package av

// Filter processes decoded frames (deinterlace, scale, pixel format convert).
type Filter interface {
	Process(frame *Frame) (*Frame, error)
	Close()
}
