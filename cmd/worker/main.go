package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/priyansx01/smartfm-lms/internal/config"
	"github.com/priyansx01/smartfm-lms/internal/event"
	"github.com/priyansx01/smartfm-lms/internal/storage"
	"github.com/priyansx01/smartfm-lms/internal/worker"
)

func main() {
	cfg := config.Load()
	log.Printf("🚀 Starting LMS FFmpeg Worker...")

	// ─── MinIO Storage ────────────────────────────────────────────────────────
	store, err := storage.NewClient(cfg.MinIO)
	if err != nil {
		log.Fatalf("❌ MinIO connection failed: %v", err)
	}

	// ─── Kafka Publisher ──────────────────────────────────────────────────────
	pub, err := event.NewKafkaPublisher([]string{cfg.Kafka.Brokers})
	if err != nil {
		log.Fatalf("❌ Kafka publisher failed: %v", err)
	}
	defer pub.Close()

	// ─── Video Processor ──────────────────────────────────────────────────────
	vp, err := worker.NewVideoProcessor(*cfg, store, pub)
	if err != nil {
		log.Fatalf("❌ Failed to create video processor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := vp.Start(ctx); err != nil {
			log.Fatalf("❌ Video processor failed: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("⏳ Shutting down worker gracefully...")
	cancel()
	log.Println("✅ Worker stopped")
}
