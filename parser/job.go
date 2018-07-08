package parser

import (
	"time"
)

type JobRequest struct {
	ID       *string
	JobType  *string
	Priority *float32
	Done     *bool
	DoneTime *time.Time
	Duration *float32 // in seconds
	AddTime  *time.Time
	Modified *time.Time
	Tryed    *int32

	HREF        *string
	Offset      *string
	TrackerName *string
	IP          *string

	TrackerTorrentID *string
	JobName          *string

	TorrentID *string
}
