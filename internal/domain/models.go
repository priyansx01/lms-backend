// Package domain defines shared models used across all services.
// These structs map directly to the PostgreSQL tables described
// in the architecture document (§6 Key Data Models).
package domain

import "time"

// ─── User ─────────────────────────────────────────────────────────────────────

// UserRole matches the JWT role claim.
type UserRole string

const (
	RoleEmployee   UserRole = "employee"
	RoleInstructor UserRole = "instructor"
	RoleAdmin      UserRole = "admin"
)

// User represents an LMS user (maps to `users` table).
type User struct {
	ID        string    `json:"lms_user_id" db:"id"`
	SmartFMID *string   `json:"smartfm_id,omitempty" db:"smartfm_id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"` // never serialized
	Role      UserRole  `json:"role" db:"role"`
	AvatarURL *string   `json:"avatar_url,omitempty" db:"avatar_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ─── Course ───────────────────────────────────────────────────────────────────

type CourseStatus string

const (
	CourseStatusPending    CourseStatus = "pending"
	CourseStatusProcessing CourseStatus = "processing"
	CourseStatusReady      CourseStatus = "ready"
	CourseStatusArchived   CourseStatus = "archived"
)

// Course represents a training course (maps to `courses` table).
type Course struct {
	ID             string       `json:"id" db:"id"`
	CreatedBy      string       `json:"created_by" db:"created_by"`
	Title          string       `json:"title" db:"title"`
	Description    string       `json:"description" db:"description"`
	Category       string       `json:"category" db:"category"`
	Level          string       `json:"level" db:"level"`
	Status         CourseStatus `json:"status" db:"status"`
	HLSUrl         *string      `json:"hls_url,omitempty" db:"hls_url"`
	ThumbnailURL   *string      `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	DurationSeconds int         `json:"duration_seconds" db:"duration_seconds"`
	CreatedAt      time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at" db:"updated_at"`
}

// ─── Module ───────────────────────────────────────────────────────────────────

// Module represents a video module within a course (maps to `modules` table).
type Module struct {
	ID              string       `json:"id" db:"id"`
	CourseID        string       `json:"course_id" db:"course_id"`
	Title           string       `json:"title" db:"title"`
	Description     string       `json:"description" db:"description"`
	DurationSeconds int          `json:"duration_seconds" db:"duration_seconds"`
	OrderIndex      int          `json:"order_index" db:"order_index"`
	HLSUrl          *string      `json:"hls_url,omitempty" db:"hls_url"`
	Status          CourseStatus `json:"status" db:"status"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`
}

// ─── Assessment ───────────────────────────────────────────────────────────────

// AssessmentResult represents a completed quiz attempt (maps to `assessment_results` table).
type AssessmentResult struct {
	ID            string    `json:"id" db:"id"`
	LMSUserID     string    `json:"lms_user_id" db:"lms_user_id"`
	CourseID      string    `json:"course_id" db:"course_id"`
	Score         int       `json:"score" db:"score"`
	Passed        bool      `json:"passed" db:"passed"`
	AttemptNumber int       `json:"attempt_number" db:"attempt_number"`
	CompletedAt   time.Time `json:"completed_at" db:"completed_at"`
}

// QuizQuestion represents a single question in a course quiz.
type QuizQuestion struct {
	ID              string       `json:"id" db:"id"`
	CourseID        string       `json:"course_id" db:"course_id"`
	Text            string       `json:"text" db:"text"`
	Options         []QuizOption `json:"options"`
	CorrectOptionID string       `json:"correct_option_id,omitempty" db:"correct_option_id"` // hidden from students
	Explanation     string       `json:"explanation,omitempty" db:"explanation"`
	OrderIndex      int          `json:"order_index" db:"order_index"`
}

// QuizOption is a single answer choice.
type QuizOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ─── Employee (User Service view) ─────────────────────────────────────────────

type EmployeeStatus string

const (
	EmployeeActive   EmployeeStatus = "active"
	EmployeeAtRisk   EmployeeStatus = "at_risk"
	EmployeeInactive EmployeeStatus = "inactive"
)

type ComplianceStatus string

const (
	ComplianceCompliant ComplianceStatus = "compliant"
	CompliancePending   ComplianceStatus = "pending"
	ComplianceOverdue   ComplianceStatus = "overdue"
)

// EmployeeProfile is an enriched view of a user with training stats.
type EmployeeProfile struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	EmployeeID       string           `json:"employee_id"`
	Role             string           `json:"role"`
	Department       string           `json:"department"`
	Site             string           `json:"site"`
	JoiningDate      string           `json:"joining_date"`
	Status           EmployeeStatus   `json:"status"`
	CoursesAssigned  int              `json:"courses_assigned"`
	CoursesCompleted int              `json:"courses_completed"`
	ComplianceStatus ComplianceStatus `json:"compliance_status"`
	AvgScore         int              `json:"avg_score"`
	TotalLearningTime float64         `json:"total_learning_time"`
	LastActive       string           `json:"last_active"`
}

// ─── Content ──────────────────────────────────────────────────────────────────

type ContentType string

const (
	ContentVideo      ContentType = "video"
	ContentAssessment ContentType = "assessment"
	ContentTest       ContentType = "test"
)

type ContentStatus string

const (
	ContentDraft     ContentStatus = "draft"
	ContentPublished ContentStatus = "published"
	ContentArchived  ContentStatus = "archived"
)

// ContentItem is a piece of learning content.
type ContentItem struct {
	ID            string        `json:"id" db:"id"`
	Title         string        `json:"title" db:"title"`
	Type          ContentType   `json:"type" db:"type"`
	Category      string        `json:"category" db:"category"`
	Duration      *int          `json:"duration,omitempty" db:"duration"`
	QuestionCount *int          `json:"question_count,omitempty" db:"question_count"`
	Status        ContentStatus `json:"status" db:"status"`
	CreatedBy     string        `json:"created_by" db:"created_by"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
}

// Category is a content classification tag.
type Category struct {
	ID   string `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// ─── Leaderboard ──────────────────────────────────────────────────────────────

// LeaderboardEntry is a single row in the leaderboard response.
type LeaderboardEntry struct {
	EmployeeID        string   `json:"employee_id"`
	Name              string   `json:"name"`
	Role              string   `json:"role"`
	Site              string   `json:"site"`
	TotalScore        float64  `json:"total_score"`
	CoursesCompleted  int      `json:"courses_completed"`
	MandatoryComplete int      `json:"mandatory_completed"`
	TotalAssigned     int      `json:"total_assigned"`
	AvgScore          float64  `json:"avg_score"`
	TotalTimeSpent    float64  `json:"total_time_spent"`
	TotalAttempts     int      `json:"total_attempts"`
	EfficiencyScore   float64  `json:"efficiency_score"`
	Rank              int      `json:"rank"`
	Badges            []string `json:"badges,omitempty"`
}

// ─── Analytics ────────────────────────────────────────────────────────────────

// OverviewMetrics is the top-level dashboard KPIs.
type OverviewMetrics struct {
	TotalEmployeesEnrolled int     `json:"total_employees_enrolled"`
	ActiveLearners         int     `json:"active_learners"`
	OverallCompletionRate  float64 `json:"overall_completion_rate"`
	AvgTimeSpentPerCourse  float64 `json:"avg_time_spent_per_course"`
	EngagementScore        float64 `json:"engagement_score"`
}
