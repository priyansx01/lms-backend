package event

// Topic names
const (
	TopicVideoUploaded  = "video.uploaded"
	TopicVideoProcessed = "video.processed"
)

// VideoUploadedEvent is the payload sent when a raw video upload completes.
type VideoUploadedEvent struct {
	CourseID       string `json:"course_id"`
	ModuleID       string `json:"module_id"`
	LMSUserID      string `json:"lms_user_id"`
	MinioKey       string `json:"minio_key"`
	QualityTargets []int  `json:"quality_targets"` // e.g. [360, 720, 1080]
	Timestamp      string `json:"timestamp"`
}

// VideoProcessedEvent is the payload sent by the FFmpeg worker upon completion.
type VideoProcessedEvent struct {
	CourseID        string `json:"course_id"`
	ModuleID        string `json:"module_id"`
	HLSUrl          string `json:"hls_url"`
	ThumbnailUrl    string `json:"thumbnail_url"`
	DurationSeconds int    `json:"duration_seconds"`
	Status          string `json:"status"` // "ready" or "error"
}
