package discounts

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	AuditEventCampaignCreated     = "campaign_created"
	AuditEventCampaignUpdated     = "campaign_updated"
	AuditEventCampaignDisabled    = "campaign_disabled"
	AuditEventCampaignArchived    = "campaign_archived"
	AuditEventScheduleUpdated     = "schedule_updated"
	AuditEventLifecycleTransition = "lifecycle_transition"
)

type EvaluationMetrics struct {
	TotalEvaluations       uint64
	FailedEvaluations      uint64
	MatchedEvaluations     uint64
	TotalLatencyMillis     uint64
	LastLatencyMillis      uint64
	LastLineCount          uint64
	LastCandidateCampaigns uint64
	LastMatchedCampaigns   uint64
	LastEvaluatedAt        time.Time
	LastError              string
}

type ReconciliationIssue struct {
	CampaignID     uint
	ScheduleID     uint
	ExpectedStatus string
	ActualStatus   string
	ExpectedStart  time.Time
	ActualStart    time.Time
	ExpectedEnd    *time.Time
	ActualEnd      *time.Time
	Message        string
}

type ReconciliationReport struct {
	CheckedAt uint64
	Issues    []ReconciliationIssue
}

type evaluationMetricState struct {
	totalEvaluations       atomic.Uint64
	failedEvaluations      atomic.Uint64
	matchedEvaluations     atomic.Uint64
	totalLatencyMillis     atomic.Uint64
	lastLatencyMillis      atomic.Uint64
	lastLineCount          atomic.Uint64
	lastCandidateCampaigns atomic.Uint64
	lastMatchedCampaigns   atomic.Uint64
	lastEvaluatedUnix      atomic.Int64
	lastError              atomic.Value
}

var metrics evaluationMetricState

func EvaluationMetricsSnapshot() EvaluationMetrics {
	var lastError string
	if value := metrics.lastError.Load(); value != nil {
		lastError, _ = value.(string)
	}
	lastUnix := metrics.lastEvaluatedUnix.Load()
	var lastEvaluatedAt time.Time
	if lastUnix > 0 {
		lastEvaluatedAt = time.Unix(0, lastUnix).UTC()
	}
	return EvaluationMetrics{
		TotalEvaluations:       metrics.totalEvaluations.Load(),
		FailedEvaluations:      metrics.failedEvaluations.Load(),
		MatchedEvaluations:     metrics.matchedEvaluations.Load(),
		TotalLatencyMillis:     metrics.totalLatencyMillis.Load(),
		LastLatencyMillis:      metrics.lastLatencyMillis.Load(),
		LastLineCount:          metrics.lastLineCount.Load(),
		LastCandidateCampaigns: metrics.lastCandidateCampaigns.Load(),
		LastMatchedCampaigns:   metrics.lastMatchedCampaigns.Load(),
		LastEvaluatedAt:        lastEvaluatedAt,
		LastError:              lastError,
	}
}

func recordEvaluationMetric(start time.Time, lineCount int, candidateCount int, result EvaluationResult, err error) {
	latency := uint64(time.Since(start).Milliseconds())
	matched := countMatchedCampaigns(result)
	metrics.totalEvaluations.Add(1)
	metrics.totalLatencyMillis.Add(latency)
	metrics.lastLatencyMillis.Store(latency)
	metrics.lastLineCount.Store(uint64(lineCount))
	metrics.lastCandidateCampaigns.Store(uint64(candidateCount))
	metrics.lastMatchedCampaigns.Store(uint64(matched))
	metrics.lastEvaluatedUnix.Store(time.Now().UTC().UnixNano())
	if matched > 0 {
		metrics.matchedEvaluations.Add(1)
	}
	if err != nil {
		metrics.failedEvaluations.Add(1)
		metrics.lastError.Store(err.Error())
		log.Printf("discount_evaluation_failed line_count=%d candidate_campaigns=%d latency_ms=%d error=%q", lineCount, candidateCount, latency, err.Error())
		return
	}
	metrics.lastError.Store("")
	log.Printf("discount_evaluation_completed line_count=%d candidate_campaigns=%d matched_campaigns=%d latency_ms=%d", lineCount, candidateCount, matched, latency)
}

func countMatchedCampaigns(result EvaluationResult) int {
	seen := map[uint]struct{}{}
	for _, line := range result.Lines {
		for _, campaign := range line.AppliedCampaigns {
			seen[campaign.ID] = struct{}{}
		}
	}
	return len(seen)
}

func ListCampaignAudits(db *gorm.DB, campaignID *uint) ([]models.DiscountCampaignAudit, error) {
	query := db.Order("changed_at DESC").Order("id DESC")
	if campaignID != nil {
		query = query.Where("campaign_id = ?", *campaignID)
	}
	var audits []models.DiscountCampaignAudit
	if err := query.Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}

func RunReconciliation(db *gorm.DB, now time.Time) (ReconciliationReport, error) {
	var schedules []models.DiscountSchedule
	if err := db.Preload("Campaign").Find(&schedules).Error; err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{CheckedAt: uint64(now.UTC().Unix())}
	for _, schedule := range schedules {
		if schedule.Campaign == nil || schedule.Campaign.ID == 0 {
			report.Issues = append(report.Issues, ReconciliationIssue{
				ScheduleID: schedule.ID,
				Message:    "schedule has no campaign",
			})
			continue
		}
		expectedStatus, expectedStart, expectedEnd, expectedArchived, err := expectedScheduleState(schedule, now.UTC())
		if err != nil {
			return ReconciliationReport{}, err
		}
		campaign := schedule.Campaign
		if campaign.Status != expectedStatus || campaign.IsArchived != expectedArchived || !campaign.StartsAt.Equal(expectedStart) || !sameTimePtr(campaign.EndsAt, expectedEnd) {
			report.Issues = append(report.Issues, ReconciliationIssue{
				CampaignID:     campaign.ID,
				ScheduleID:     schedule.ID,
				ExpectedStatus: expectedStatus,
				ActualStatus:   campaign.Status,
				ExpectedStart:  expectedStart,
				ActualStart:    campaign.StartsAt,
				ExpectedEnd:    expectedEnd,
				ActualEnd:      campaign.EndsAt,
				Message:        "campaign runtime state does not match schedule",
			})
		}
	}
	return report, nil
}

func expectedScheduleState(schedule models.DiscountSchedule, now time.Time) (string, time.Time, *time.Time, bool, error) {
	window, err := resolveScheduleWindow(schedule.ScheduleType, schedule.RRule, schedule.WindowStart, schedule.WindowEnd, schedule.UntilAt, now)
	if err != nil {
		return "", time.Time{}, nil, false, err
	}
	if window.Expired {
		return models.DiscountCampaignStatusArchived, schedule.WindowStart, &schedule.WindowEnd, true, nil
	}
	if !window.Start.IsZero() && !now.Before(window.Start) && now.Before(window.End) {
		return models.DiscountCampaignStatusActive, window.Start, &window.End, false, nil
	}
	start := schedule.WindowStart
	end := &schedule.WindowEnd
	if window.NextStart != nil {
		start = *window.NextStart
		end = window.NextEnd
	}
	return models.DiscountCampaignStatusScheduled, start, end, false, nil
}

func createCampaignAudit(tx *gorm.DB, campaignID uint, eventType string, source string, actor string, summary string, before any, after any, now time.Time) error {
	beforeJSON, err := encodeAuditJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := encodeAuditJSON(after)
	if err != nil {
		return err
	}
	return tx.Create(&models.DiscountCampaignAudit{
		CampaignID: campaignID,
		EventType:  eventType,
		Source:     source,
		Actor:      actor,
		Summary:    summary,
		BeforeJSON: beforeJSON,
		AfterJSON:  afterJSON,
		ChangedAt:  now.UTC(),
	}).Error
}

func encodeAuditJSON(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode discount audit payload: %w", err)
	}
	return string(raw), nil
}
