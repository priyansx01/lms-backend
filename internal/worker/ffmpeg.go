package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"

	"github.com/priyansx01/smartfm-lms/internal/config"
	"github.com/priyansx01/smartfm-lms/internal/event"
	"github.com/priyansx01/smartfm-lms/internal/storage"
)

type VideoProcessor struct {
	cfg     config.Config
	storage *storage.Client
	rdb     *redis.Client
	sub     message.Subscriber
	pub     *event.Publisher
}

func NewVideoProcessor(cfg config.Config, st *storage.Client, rdb *redis.Client, p *event.Publisher) (*VideoProcessor, error) {
	sub, err := kafka.NewSubscriber(
		kafka.SubscriberConfig{
			Brokers:       []string{cfg.Kafka.Brokers},
			Unmarshaler:   kafka.DefaultMarshaler{},
			ConsumerGroup: "ffmpeg-workers",
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("create subscriber: %w", err)
	}

	return &VideoProcessor{
		cfg:     cfg,
		storage: st,
		rdb:     rdb,
		sub:     sub,
		pub:     p,
	}, nil
}

func (vp *VideoProcessor) Start(ctx context.Context) error {
	messages, err := vp.sub.Subscribe(ctx, event.TopicVideoUploaded)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", event.TopicVideoUploaded, err)
	}

	log.Printf("🎥 VideoProcessor started. Listening to %s...", event.TopicVideoUploaded)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping video processor...")
			return vp.sub.Close()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			vp.handleMessage(ctx, msg)
		}
	}
}

func (vp *VideoProcessor) handleMessage(ctx context.Context, msg *message.Message) {
	var ev event.VideoUploadedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		log.Printf("❌ Failed to unmarshal message: %v", err)
		msg.Ack()
		return
	}

	log.Printf("⚙️ Processing video for course %s, module %s...", ev.CourseID, ev.ModuleID)
	vp.updateProgress(ctx, ev.ModuleID, 0, "downloading")

	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ffmpeg-lms-*")
	if err != nil {
		log.Printf("❌ Failed to create temp dir: %v", err)
		msg.Nack()
		return
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.mp4")

	// 2. Download video
	if err := vp.storage.DownloadRawFile(ctx, ev.MinioKey, inputPath); err != nil {
		log.Printf("❌ Failed to download raw file: %v", err)
		msg.Nack()
		return
	}

	vp.updateProgress(ctx, ev.ModuleID, 5, "probing")

	// 3. Get duration
	durationStr, err := getDuration(inputPath)
	if err != nil {
		log.Printf("❌ Failed to get duration: %v", err)
		msg.Nack()
		return
	}
	totalSeconds, _ := strconv.ParseFloat(durationStr, 64)

	vp.updateProgress(ctx, ev.ModuleID, 10, "transcoding")

	// 4. Transcode
	outDir := filepath.Join(tmpDir, "hls")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath,
		"-filter_complex", "[0:v]split=4[v1080][v720][v480][v360]; [v1080]scale=w=1920:h=1080:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v1080out]; [v720]scale=w=1280:h=720:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v720out]; [v480]scale=w=854:h=480:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v480out]; [v360]scale=w=640:h=360:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2[v360out]",
		"-map", "[v1080out]", "-map", "0:a", "-c:v:0", "libx264", "-preset", "ultrafast", "-profile:v:0", "high", "-crf", "20", "-maxrate:v:0", "5350k", "-bufsize:v:0", "7500k", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
		"-map", "[v720out]", "-map", "0:a", "-c:v:1", "libx264", "-preset", "ultrafast", "-profile:v:1", "high", "-crf", "21", "-maxrate:v:1", "3000k", "-bufsize:v:1", "4200k", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
		"-map", "[v480out]", "-map", "0:a", "-c:v:2", "libx264", "-preset", "ultrafast", "-profile:v:2", "main", "-crf", "22", "-maxrate:v:2", "1500k", "-bufsize:v:2", "2100k", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
		"-map", "[v360out]", "-map", "0:a", "-c:v:3", "libx264", "-preset", "ultrafast", "-profile:v:3", "baseline", "-crf", "23", "-maxrate:v:3", "856k", "-bufsize:v:3", "1200k", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-f", "hls", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments", "-hls_segment_type", "mpegts",
		"-hls_segment_filename", filepath.Join(outDir, "v%v_segment_%03d.ts"),
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", "v:0,a:0 v:1,a:1 v:2,a:2 v:3,a:3",
		filepath.Join(outDir, "v%v_index.m3u8"),
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("❌ Failed to pipe stderr: %v", err)
		msg.Nack()
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("❌ Failed to start ffmpeg: %v", err)
		msg.Nack()
		return
	}

	timeRe := regexp.MustCompile(`time=([0-9]{2}):([0-9]{2}):([0-9]{2}\.[0-9]{2})`)
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		matches := timeRe.FindStringSubmatch(line)
		if len(matches) == 4 {
			h, _ := strconv.ParseFloat(matches[1], 64)
			m, _ := strconv.ParseFloat(matches[2], 64)
			s, _ := strconv.ParseFloat(matches[3], 64)
			currentTime := h*3600 + m*60 + s
			if totalSeconds > 0 {
				progress := 10 + int((currentTime/totalSeconds)*80) // 10% to 90%
				if progress > 90 {
					progress = 90
				}
				vp.updateProgress(ctx, ev.ModuleID, progress, "transcoding")
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("❌ FFmpeg failed: %v", err)
		msg.Nack()
		return
	}

	vp.updateProgress(ctx, ev.ModuleID, 90, "thumbnailing")

	// 5. Generate Thumbnail
	thumbPath := filepath.Join(tmpDir, "thumb.jpg")
	exec.Command("ffmpeg", "-y", "-i", inputPath, "-ss", "00:00:01.000", "-vframes", "1", thumbPath).Run()

	vp.updateProgress(ctx, ev.ModuleID, 95, "uploading")

	// 6. Upload files to MinIO
	hlsPrefix := fmt.Sprintf("courses/%s/%s", ev.CourseID, ev.ModuleID)
	files, _ := filepath.Glob(filepath.Join(outDir, "*"))
	for _, f := range files {
		fileName := filepath.Base(f)
		objectKey := fmt.Sprintf("%s/%s", hlsPrefix, fileName)
		contentType := "video/MP2T"
		if strings.HasSuffix(fileName, ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		}
		vp.storage.UploadHLSFile(ctx, objectKey, f, contentType)
	}

	thumbObjectKey := fmt.Sprintf("courses/%s/%s/thumb.jpg", ev.CourseID, ev.ModuleID)
	vp.storage.UploadThumbnail(ctx, thumbObjectKey, thumbPath, "image/jpeg")

	// 7. Publish success
	hlsURL := fmt.Sprintf("http://localhost:9002/%s/courses/%s/%s/master.m3u8", vp.cfg.MinIO.HLSBucket, ev.CourseID, ev.ModuleID)
	thumbnailURL := fmt.Sprintf("http://localhost:9000/%s/courses/%s/%s/thumb.jpg", vp.cfg.MinIO.ThumbnailsBucket, ev.CourseID, ev.ModuleID)

	processedEv := event.VideoProcessedEvent{
		CourseID:        ev.CourseID,
		ModuleID:        ev.ModuleID,
		HLSUrl:          hlsURL,
		ThumbnailUrl:    thumbnailURL,
		DurationSeconds: int(totalSeconds),
		Status:          "ready",
	}

	if vp.pub != nil {
		if err := vp.pub.PublishJSON(ctx, event.TopicVideoProcessed, processedEv); err != nil {
			log.Printf("❌ Failed to publish processed event: %v", err)
			msg.Nack()
			return
		}
	}

	vp.updateProgress(ctx, ev.ModuleID, 100, "ready")
	log.Printf("✅ Processing complete for course %s, module %s.", ev.CourseID, ev.ModuleID)
	msg.Ack()
}

func (vp *VideoProcessor) updateProgress(ctx context.Context, moduleID string, percent int, status string) {
	key := fmt.Sprintf("module:progress:%s", moduleID)
	data, _ := json.Marshal(map[string]interface{}{
		"percent": percent,
		"status":  status,
	})
	vp.rdb.Set(ctx, key, data, 24*time.Hour)
}

func getDuration(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
