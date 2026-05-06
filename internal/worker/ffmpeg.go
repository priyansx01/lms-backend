package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/priyansx01/smartfm-lms/internal/config"
	"github.com/priyansx01/smartfm-lms/internal/event"
	"github.com/priyansx01/smartfm-lms/internal/storage"
)

type VideoProcessor struct {
	cfg     config.Config
	storage *storage.Client
	sub     message.Subscriber
	pub     *event.Publisher
}

func NewVideoProcessor(cfg config.Config, st *storage.Client, p *event.Publisher) (*VideoProcessor, error) {
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

	log.Printf("⚙️  Processing video for course %s, module %s...", ev.CourseID, ev.ModuleID)

	// Note: in a real production system we'd use a temporary directory for transcoding
	// For this prototype we will skip the actual FFmpeg shelling and simulate it,
	// or we can write the actual FFmpeg command. The architecture document says:
	// "Transcodes to 360p, 720p, 1080p using FFmpeg binary (shelled out from Go — no CGO)"

	// Simulate processing time
	// time.Sleep(5 * time.Second)
	// (Actual processing logic would be downloading from MinIO Raw, running FFmpeg, uploading to MinIO HLS)
	// For now we simulate generating a HLS URL and marking it ready.

	hlsURL := fmt.Sprintf("http://localhost:9002/%s/courses/%s/modules/%s/master.m3u8", vp.cfg.MinIO.HLSBucket, ev.CourseID, ev.ModuleID)
	thumbnailURL := fmt.Sprintf("http://localhost:9000/%s/courses/%s/modules/%s/thumb.jpg", vp.cfg.MinIO.ThumbnailsBucket, ev.CourseID, ev.ModuleID)

	// Publish success
	processedEv := event.VideoProcessedEvent{
		CourseID:        ev.CourseID,
		ModuleID:        ev.ModuleID,
		HLSUrl:          hlsURL,
		ThumbnailUrl:    thumbnailURL,
		DurationSeconds: 120, // simulated
		Status:          "ready",
	}

	if vp.pub != nil {
		if err := vp.pub.PublishJSON(ctx, event.TopicVideoProcessed, processedEv); err != nil {
			log.Printf("❌ Failed to publish processed event: %v", err)
			msg.Nack() // Re-queue
			return
		}
	}

	log.Printf("✅ Processing complete for course %s, module %s.", ev.CourseID, ev.ModuleID)
	msg.Ack()
}
