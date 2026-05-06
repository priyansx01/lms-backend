// Package course handles course and module CRUD.
package course

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/priyansx01/smartfm-lms/internal/domain"
	"github.com/priyansx01/smartfm-lms/internal/event"
	"github.com/priyansx01/smartfm-lms/internal/storage"
)

var (
	ErrCourseNotFound = errors.New("course not found")
	ErrModuleNotFound = errors.New("module not found")
)

// Service provides course business logic.
type Service struct {
	db      *sql.DB
	storage *storage.Client
	pub     *event.Publisher
}

// NewService creates a new course service.
func NewService(db *sql.DB, s *storage.Client, p *event.Publisher) *Service {
	return &Service{db: db, storage: s, pub: p}
}

// ─── Course CRUD ──────────────────────────────────────────────────────────────

// CreateCourseRequest is the payload for POST /courses.
type CreateCourseRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Level       string   `json:"level"`
	Tags        []string `json:"tags,omitempty"`
}

// ListCourses returns all courses with optional filtering.
func (s *Service) ListCourses(status, search, category string) ([]domain.Course, error) {
	query := `SELECT id, created_by, title, description, category, level, status, 
	          hls_url, thumbnail_url, duration_seconds, created_at, updated_at 
	          FROM courses WHERE 1=1`
	args := []interface{}{}
	idx := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if search != "" {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}
	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", idx)
		args = append(args, category)
		idx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()

	var courses []domain.Course
	for rows.Next() {
		var c domain.Course
		if err := rows.Scan(
			&c.ID, &c.CreatedBy, &c.Title, &c.Description, &c.Category,
			&c.Level, &c.Status, &c.HLSUrl, &c.ThumbnailURL,
			&c.DurationSeconds, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		courses = append(courses, c)
	}
	return courses, nil
}

// GetCourse returns a single course by ID.
func (s *Service) GetCourse(id string) (*domain.Course, error) {
	var c domain.Course
	err := s.db.QueryRow(`
		SELECT id, created_by, title, description, category, level, status,
		       hls_url, thumbnail_url, duration_seconds, created_at, updated_at
		FROM courses WHERE id = $1
	`, id).Scan(
		&c.ID, &c.CreatedBy, &c.Title, &c.Description, &c.Category,
		&c.Level, &c.Status, &c.HLSUrl, &c.ThumbnailURL,
		&c.DurationSeconds, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}
	return &c, nil
}

// CreateCourse inserts a new course with status "pending".
func (s *Service) CreateCourse(userID string, req CreateCourseRequest) (*domain.Course, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO courses (id, created_by, title, description, category, level, status, duration_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, id, userID, req.Title, req.Description, req.Category, req.Level, domain.CourseStatusPending, 0, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert course: %w", err)
	}

	return s.GetCourse(id)
}

// UpdateCourse patches course metadata.
func (s *Service) UpdateCourse(id string, req CreateCourseRequest) (*domain.Course, error) {
	_, err := s.db.Exec(`
		UPDATE courses SET title = $1, description = $2, category = $3, level = $4, updated_at = $5
		WHERE id = $6
	`, req.Title, req.Description, req.Category, req.Level, time.Now(), id)
	if err != nil {
		return nil, fmt.Errorf("update course: %w", err)
	}
	return s.GetCourse(id)
}

// DeleteCourse soft-deletes by setting status to archived.
func (s *Service) DeleteCourse(id string) error {
	_, err := s.db.Exec(`UPDATE courses SET status = $1, updated_at = $2 WHERE id = $3`,
		domain.CourseStatusArchived, time.Now(), id)
	return err
}

// ─── Module CRUD ──────────────────────────────────────────────────────────────

// CreateModuleRequest is the payload for creating a module.
type CreateModuleRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	OrderIndex  int    `json:"order_index"`
}

// ListModules returns all modules for a course ordered by order_index.
func (s *Service) ListModules(courseID string) ([]domain.Module, error) {
	rows, err := s.db.Query(`
		SELECT id, course_id, title, description, duration_seconds, order_index, hls_url, status, created_at, updated_at
		FROM modules WHERE course_id = $1 ORDER BY order_index
	`, courseID)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	defer rows.Close()

	var modules []domain.Module
	for rows.Next() {
		var m domain.Module
		if err := rows.Scan(
			&m.ID, &m.CourseID, &m.Title, &m.Description, &m.DurationSeconds,
			&m.OrderIndex, &m.HLSUrl, &m.Status, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan module: %w", err)
		}
		modules = append(modules, m)
	}
	return modules, nil
}

// CreateModule adds a module to a course.
func (s *Service) CreateModule(courseID string, req CreateModuleRequest) (*domain.Module, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO modules (id, course_id, title, description, duration_seconds, order_index, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, courseID, req.Title, req.Description, 0, req.OrderIndex, domain.CourseStatusPending, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert module: %w", err)
	}

	var m domain.Module
	err = s.db.QueryRow(`
		SELECT id, course_id, title, description, duration_seconds, order_index, hls_url, status, created_at, updated_at
		FROM modules WHERE id = $1
	`, id).Scan(&m.ID, &m.CourseID, &m.Title, &m.Description, &m.DurationSeconds,
		&m.OrderIndex, &m.HLSUrl, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get module: %w", err)
	}
	return &m, nil
}

// DeleteModule removes a module.
func (s *Service) DeleteModule(courseID, moduleID string) error {
	_, err := s.db.Exec(`DELETE FROM modules WHERE id = $1 AND course_id = $2`, moduleID, courseID)
	return err
}

// ─── Upload URL ───────────────────────────────────────────────────────────────

// UploadURLRequest is the payload for requesting a presigned upload URL.
type UploadURLRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
}

// GetUploadURL returns a presigned MinIO URL for direct browser upload.
func (s *Service) GetUploadURL(courseID, moduleID, fileName string) (string, string, error) {
	objectKey := fmt.Sprintf("raw/%s/%s/%s", courseID, moduleID, fileName)
	url, err := s.storage.PresignedUpload(objectKey, 1*time.Hour)
	if err != nil {
		return "", "", err
	}
	return url, objectKey, nil
}

// GetPlaybackURL returns a presigned playback URL for an HLS stream.
func (s *Service) GetPlaybackURL(courseID, moduleID string) (string, time.Time, error) {
	ttl := 4 * time.Hour
	url, err := s.storage.HLSPlaybackURL(courseID, moduleID, ttl)
	if err != nil {
		return "", time.Time{}, err
	}
	return url, time.Now().Add(ttl), nil
}

// UploadModuleFile handles direct file upload and triggers processing.
func (s *Service) UploadModuleFile(ctx context.Context, userID, courseID, moduleID, fileName string, reader io.Reader, size int64, contentType string) error {
	objectKey := fmt.Sprintf("raw/%s/%s/%s", courseID, moduleID, fileName)

	err := s.storage.UploadFile(ctx, objectKey, reader, size, contentType)
	if err != nil {
		return fmt.Errorf("storage upload: %w", err)
	}

	// Update module status
	_, err = s.db.Exec(`UPDATE modules SET status = $1, updated_at = $2 WHERE id = $3 AND course_id = $4`,
		"processing", time.Now(), moduleID, courseID)
	if err != nil {
		return fmt.Errorf("update module status: %w", err)
	}

	// Fire event to Kafka if configured
	if s.pub != nil {
		ev := event.VideoUploadedEvent{
			CourseID:       courseID,
			ModuleID:       moduleID,
			LMSUserID:      userID,
			MinioKey:       objectKey,
			QualityTargets: []int{360, 720, 1080},
			Timestamp:      time.Now().Format(time.RFC3339),
		}
		if err := s.pub.PublishJSON(ctx, event.TopicVideoUploaded, ev); err != nil {
			return fmt.Errorf("publish video.uploaded event: %w", err)
		}
	}

	return nil
}

// CompleteUpload handles post-upload logic: updating DB and firing the Kafka event.
func (s *Service) CompleteUpload(ctx context.Context, userID, courseID, moduleID, fileName string) error {
	objectKey := fmt.Sprintf("raw/%s/%s/%s", courseID, moduleID, fileName)

	// Update module status
	_, err := s.db.Exec(`UPDATE modules SET status = $1, updated_at = $2 WHERE id = $3 AND course_id = $4`,
		"processing", time.Now(), moduleID, courseID)
	if err != nil {
		return fmt.Errorf("update module status: %w", err)
	}

	// Fire event to Kafka if configured
	if s.pub != nil {
		ev := event.VideoUploadedEvent{
			CourseID:       courseID,
			ModuleID:       moduleID,
			LMSUserID:      userID,
			MinioKey:       objectKey,
			QualityTargets: []int{360, 720, 1080},
			Timestamp:      time.Now().Format(time.RFC3339),
		}
		if err := s.pub.PublishJSON(ctx, event.TopicVideoUploaded, ev); err != nil {
			// Log error but don't fail the upload complete request, maybe queue it
			// For now just returning the error
			return fmt.Errorf("publish video.uploaded event: %w", err)
		}
	}

	return nil
}
