package search

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
)

const JobTypeAnalyticsRetention = "search.analytics_retention"

func (s *Service) RegisterAnalyticsJobHandler() error {
	if s == nil || s.jobs == nil {
		return errors.New("search job runtime is required")
	}
	return s.jobs.Register(JobTypeAnalyticsRetention, jobs.Registration{Handle: s.handleAnalyticsRetention})
}
func (s *Service) EnqueueAnalyticsRetention(ctx context.Context) (models.JobQueue, error) {
	if s == nil || s.jobs == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	at := s.now().UTC().Truncate(24 * time.Hour)
	return s.jobs.Enqueue(ctx, jobs.EnqueueInput{JobType: JobTypeAnalyticsRetention, Payload: struct {
		Version int `json:"version"`
	}{1}, IdempotencyKey: "analytics-retention:" + at.Format("2006-01-02")})
}
func (s *Service) handleAnalyticsRetention(ctx context.Context, raw json.RawMessage) error {
	var payload struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != 1 {
		return reliability.Classify(errors.New("invalid analytics retention payload"), reliability.ClassTerminal)
	}
	if err := s.PruneAnalytics(ctx); err != nil {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	return nil
}
