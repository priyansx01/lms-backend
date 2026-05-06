package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

type Service struct {
	db driver.Conn
}

func NewService(db driver.Conn) *Service {
	return &Service{db: db}
}

// TrackVideoWatched logs a video watch event.
func (s *Service) TrackVideoWatched(ctx context.Context, userID, courseID, moduleID string, watchPct float32) error {
	uUUID, _ := uuid.Parse(userID)
	cUUID, _ := uuid.Parse(courseID)
	mUUID, _ := uuid.Parse(moduleID)

	err := s.db.Exec(ctx, `
		INSERT INTO video_watched (user_id, course_id, module_id, watch_pct, timestamp) 
		VALUES (?, ?, ?, ?, ?)
	`, uUUID, cUUID, mUUID, watchPct, time.Now())

	if err != nil {
		return fmt.Errorf("track video watched: %w", err)
	}
	return nil
}

// TrackQuizAttempted logs a quiz attempt event.
func (s *Service) TrackQuizAttempted(ctx context.Context, userID, courseID string, score int, passed bool) error {
	uUUID, _ := uuid.Parse(userID)
	cUUID, _ := uuid.Parse(courseID)
	var p uint8
	if passed {
		p = 1
	}

	err := s.db.Exec(ctx, `
		INSERT INTO quiz_attempted (user_id, course_id, score, passed, timestamp) 
		VALUES (?, ?, ?, ?, ?)
	`, uUUID, cUUID, int32(score), p, time.Now())

	if err != nil {
		return fmt.Errorf("track quiz attempted: %w", err)
	}
	return nil
}

// TrackDropOff logs when a user drops off from a video.
func (s *Service) TrackDropOff(ctx context.Context, userID, moduleID string, secondsWatched int) error {
	uUUID, _ := uuid.Parse(userID)
	mUUID, _ := uuid.Parse(moduleID)

	err := s.db.Exec(ctx, `
		INSERT INTO drop_off_at (user_id, module_id, seconds_watched, timestamp) 
		VALUES (?, ?, ?, ?)
	`, uUUID, mUUID, int32(secondsWatched), time.Now())

	if err != nil {
		return fmt.Errorf("track drop off: %w", err)
	}
	return nil
}

// GetModuleAvgWatchPct returns the average watch percentage for a specific module.
func (s *Service) GetModuleAvgWatchPct(ctx context.Context, moduleID string) (float32, error) {
	mUUID, err := uuid.Parse(moduleID)
	if err != nil {
		return 0, fmt.Errorf("invalid module id: %w", err)
	}

	var avgPct float32
	err = s.db.QueryRow(ctx, `
		SELECT avg(watch_pct) FROM video_watched WHERE module_id = ?
	`, mUUID).Scan(&avgPct)

	if err != nil {
		return 0, fmt.Errorf("query avg watch pct: %w", err)
	}
	return avgPct, nil
}
