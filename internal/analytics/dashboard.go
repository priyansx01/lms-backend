package analytics

import (
	"context"
	"fmt"
)

// OverviewMetrics represents high-level LMS metrics
type OverviewMetrics struct {
	TotalEmployeesEnrolled int     `json:"totalEmployeesEnrolled"`
	ActiveLearners         int     `json:"activeLearners"`
	OverallCompletionRate  float32 `json:"overallCompletionRate"`
	AvgTimeSpentPerCourse  float32 `json:"avgTimeSpentPerCourse"`
	EngagementScore        float32 `json:"engagementScore"`
}

// GetOverviewMetrics calculates basic overview stats from clickhouse.
func (s *Service) GetOverviewMetrics(ctx context.Context) (*OverviewMetrics, error) {
	var activeLearners uint64
	// Active learners = distinct users who watched video or attempted quiz in last 30 days
	err := s.db.QueryRow(ctx, `
		SELECT count(distinct user_id) 
		FROM video_watched 
		WHERE timestamp >= now() - INTERVAL 30 DAY
	`).Scan(&activeLearners)
	
	if err != nil {
		activeLearners = 0 // default if table empty or err
	}

	var overallPassRate float64
	err = s.db.QueryRow(ctx, `
		SELECT ifNaN(avg(passed), 0) * 100 
		FROM quiz_attempted
	`).Scan(&overallPassRate)
	if err != nil {
		overallPassRate = 0
	}

	return &OverviewMetrics{
		TotalEmployeesEnrolled: 0, // This would ideally come from Postgres users table
		ActiveLearners:         int(activeLearners),
		OverallCompletionRate:  float32(overallPassRate),
		AvgTimeSpentPerCourse:  0, // Requires more complex aggregation
		EngagementScore:        float32(activeLearners * 10), // Simple formula
	}, nil
}

type VideoDropoffPoint struct {
	Timestamp string `json:"timestamp"`
	Viewers   int    `json:"viewers"`
}

func (s *Service) GetVideoDropoffData(ctx context.Context) ([]VideoDropoffPoint, error) {
	// Simple aggregate rounding seconds to minute buckets (e.g. 0, 60, 120)
	rows, err := s.db.Query(ctx, `
		SELECT 
			concat(toString(intDiv(seconds_watched, 60)), ':00') as ts,
			count(distinct user_id) as viewers
		FROM drop_off_at
		GROUP BY intDiv(seconds_watched, 60)
		ORDER BY intDiv(seconds_watched, 60)
		LIMIT 20
	`)
	if err != nil {
		return nil, fmt.Errorf("query dropoff data: %w", err)
	}
	defer rows.Close()

	var res []VideoDropoffPoint
	for rows.Next() {
		var dp VideoDropoffPoint
		if err := rows.Scan(&dp.Timestamp, &dp.Viewers); err != nil {
			return nil, err
		}
		res = append(res, dp)
	}
	return res, nil
}
