package discounts

import (
	"fmt"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LifecycleSourceScheduler = "scheduler"
	LifecycleSourceAdmin     = "admin"

	HistoryReasonScheduled   = "scheduled"
	HistoryReasonActivated   = "activated"
	HistoryReasonDeactivated = "deactivated"
	HistoryReasonArchived    = "archived"
)

type ScheduleInput struct {
	ScheduleType string
	Recurrence   string
	WindowStart  time.Time
	WindowEnd    time.Time
	UntilAt      *time.Time
	Timezone     string
	Actor        string
}

type LifecycleResult struct {
	Activated   int
	Deactivated int
	Archived    int
}

type scheduleWindow struct {
	Start     time.Time
	End       time.Time
	NextStart *time.Time
	NextEnd   *time.Time
	Expired   bool
}

func UpsertSchedule(db *gorm.DB, campaignID uint, input ScheduleInput, now time.Time) (models.DiscountSchedule, error) {
	if err := validateSchedule(input); err != nil {
		return models.DiscountSchedule{}, err
	}
	var schedule models.DiscountSchedule
	err := db.Transaction(func(tx *gorm.DB) error {
		var campaign models.DiscountCampaign
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&campaign, campaignID).Error; err != nil {
			return err
		}

		window, err := resolveScheduleWindow(input.ScheduleType, input.Recurrence, input.WindowStart.UTC(), input.WindowEnd.UTC(), utcTimePtr(input.UntilAt), now.UTC())
		if err != nil {
			return err
		}
		schedule = models.DiscountSchedule{
			CampaignID:   campaignID,
			ScheduleType: input.ScheduleType,
			RRule:        normalizeRecurrence(input.Recurrence),
			WindowStart:  input.WindowStart.UTC(),
			WindowEnd:    input.WindowEnd.UTC(),
			UntilAt:      utcTimePtr(input.UntilAt),
			Timezone:     normalizeTimezone(input.Timezone),
			NextRunAt:    window.NextStart,
		}

		var existing models.DiscountSchedule
		err = tx.Where("campaign_id = ?", campaignID).First(&existing).Error
		switch err {
		case nil:
			beforeSchedule := existing
			schedule.ID = existing.ID
			if err := tx.Model(&models.DiscountSchedule{}).Where("id = ?", existing.ID).Updates(schedule).Error; err != nil {
				return err
			}
			if err := createCampaignAudit(tx, campaignID, AuditEventScheduleUpdated, LifecycleSourceAdmin, input.Actor, "updated campaign schedule", beforeSchedule, schedule, now.UTC()); err != nil {
				return err
			}
		case gorm.ErrRecordNotFound:
			if err := tx.Create(&schedule).Error; err != nil {
				return err
			}
			if err := createCampaignAudit(tx, campaignID, AuditEventScheduleUpdated, LifecycleSourceAdmin, input.Actor, "created campaign schedule", nil, schedule, now.UTC()); err != nil {
				return err
			}
		default:
			return err
		}

		toStatus := models.DiscountCampaignStatusScheduled
		startsAt := schedule.WindowStart
		endsAt := &schedule.WindowEnd
		if !window.Start.IsZero() {
			startsAt = window.Start
			endsAt = &window.End
		}
		if !window.Start.IsZero() && !now.UTC().Before(window.Start) && now.UTC().Before(window.End) {
			toStatus = models.DiscountCampaignStatusActive
		}
		if window.Expired {
			toStatus = models.DiscountCampaignStatusArchived
		}
		return transitionCampaign(tx, &campaign, toStatus, startsAt, endsAt, toStatus == models.DiscountCampaignStatusArchived, HistoryReasonScheduled, LifecycleSourceAdmin, input.Actor, now.UTC())
	})
	if err != nil {
		return models.DiscountSchedule{}, err
	}
	return schedule, nil
}

func RunLifecycle(db *gorm.DB, now time.Time) (LifecycleResult, error) {
	var schedules []models.DiscountSchedule
	if err := db.Preload("Campaign").Find(&schedules).Error; err != nil {
		return LifecycleResult{}, err
	}
	var result LifecycleResult
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, schedule := range schedules {
			var campaign models.DiscountCampaign
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&campaign, schedule.CampaignID).Error; err != nil {
				return err
			}
			if campaign.Status == models.DiscountCampaignStatusDisabled || campaign.Status == models.DiscountCampaignStatusArchived {
				continue
			}
			window, err := resolveScheduleWindow(schedule.ScheduleType, schedule.RRule, schedule.WindowStart, schedule.WindowEnd, schedule.UntilAt, now.UTC())
			if err != nil {
				return err
			}
			nextRun := window.NextStart
			lastRun := schedule.LastRunAt
			toStatus := campaign.Status
			reason := ""
			isArchived := campaign.IsArchived
			startsAt := campaign.StartsAt
			endsAt := campaign.EndsAt

			switch {
			case window.Expired:
				toStatus = models.DiscountCampaignStatusArchived
				isArchived = true
				reason = HistoryReasonArchived
				result.Archived++
			case !window.Start.IsZero() && !now.UTC().Before(window.Start) && now.UTC().Before(window.End):
				toStatus = models.DiscountCampaignStatusActive
				startsAt = window.Start
				endsAt = &window.End
				lastRun = ptrTime(now.UTC())
				reason = HistoryReasonActivated
				if campaign.Status != toStatus {
					result.Activated++
				}
			default:
				toStatus = models.DiscountCampaignStatusScheduled
				if window.NextStart != nil {
					startsAt = *window.NextStart
					endsAt = window.NextEnd
				}
				reason = HistoryReasonDeactivated
				if campaign.Status == models.DiscountCampaignStatusActive {
					result.Deactivated++
				}
			}

			if err := transitionCampaign(tx, &campaign, toStatus, startsAt, endsAt, isArchived, reason, LifecycleSourceScheduler, "", now.UTC()); err != nil {
				return err
			}
			if err := tx.Model(&models.DiscountSchedule{}).
				Where("id = ?", schedule.ID).
				Updates(map[string]any{
					"last_run_at": lastRun,
					"next_run_at": nextRun,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func ArchiveCampaign(db *gorm.DB, campaignID uint, actor string, now time.Time) (models.DiscountCampaign, error) {
	var campaign models.DiscountCampaign
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Targets").Preload("Rules").Preload("Levels").First(&campaign, campaignID).Error; err != nil {
			return err
		}
		return transitionCampaign(tx, &campaign, models.DiscountCampaignStatusArchived, campaign.StartsAt, campaign.EndsAt, true, HistoryReasonArchived, LifecycleSourceAdmin, actor, now.UTC())
	})
	if err != nil {
		return models.DiscountCampaign{}, err
	}
	return LoadCampaign(db, campaignID)
}

func ListHistory(db *gorm.DB, campaignID *uint) ([]models.DiscountStateHistory, error) {
	query := db.Order("changed_at DESC").Order("id DESC")
	if campaignID != nil {
		query = query.Where("campaign_id = ?", *campaignID)
	}
	var history []models.DiscountStateHistory
	if err := query.Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

func transitionCampaign(
	tx *gorm.DB,
	campaign *models.DiscountCampaign,
	toStatus string,
	startsAt time.Time,
	endsAt *time.Time,
	isArchived bool,
	reason string,
	source string,
	actor string,
	now time.Time,
) error {
	fromStatus := campaign.Status
	fromStartsAt := campaign.StartsAt
	fromEndsAt := campaign.EndsAt
	fromArchived := campaign.IsArchived
	changed := fromStatus != toStatus || campaign.IsArchived != isArchived || !campaign.StartsAt.Equal(startsAt) || !sameTimePtr(campaign.EndsAt, endsAt)
	if !changed {
		return nil
	}
	if err := tx.Model(&models.DiscountCampaign{}).
		Where("id = ?", campaign.ID).
		Updates(map[string]any{
			"status":      toStatus,
			"starts_at":   startsAt.UTC(),
			"ends_at":     utcTimePtr(endsAt),
			"is_archived": isArchived,
			"updated_by":  campaign.UpdatedBy,
		}).Error; err != nil {
		return err
	}
	campaign.Status = toStatus
	campaign.StartsAt = startsAt.UTC()
	campaign.EndsAt = utcTimePtr(endsAt)
	campaign.IsArchived = isArchived
	if err := tx.Create(&models.DiscountStateHistory{
		CampaignID: campaign.ID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     reason,
		Source:     source,
		Actor:      actor,
		ChangedAt:  now.UTC(),
	}).Error; err != nil {
		return err
	}
	eventType := AuditEventLifecycleTransition
	if toStatus == models.DiscountCampaignStatusArchived {
		eventType = AuditEventCampaignArchived
	}
	return createCampaignAudit(tx, campaign.ID, eventType, source, actor, reason, map[string]any{
		"status":      fromStatus,
		"starts_at":   fromStartsAt,
		"ends_at":     fromEndsAt,
		"is_archived": fromArchived,
	}, map[string]any{
		"status":      toStatus,
		"starts_at":   campaign.StartsAt,
		"ends_at":     campaign.EndsAt,
		"is_archived": isArchived,
	}, now.UTC())
}

func resolveScheduleWindow(scheduleType, recurrence string, windowStart, windowEnd time.Time, untilAt *time.Time, now time.Time) (scheduleWindow, error) {
	if !windowEnd.After(windowStart) {
		return scheduleWindow{}, fmt.Errorf("%w: window_end must be after window_start", ErrInvalidCampaign)
	}
	switch scheduleType {
	case models.DiscountScheduleTypeOneTime:
		if now.Before(windowStart) {
			return scheduleWindow{NextStart: &windowStart, NextEnd: &windowEnd}, nil
		}
		if now.Before(windowEnd) {
			return scheduleWindow{Start: windowStart, End: windowEnd}, nil
		}
		return scheduleWindow{Expired: true}, nil
	case models.DiscountScheduleTypeRecurring:
		return recurringWindow(recurrence, windowStart, windowEnd, untilAt, now)
	default:
		return scheduleWindow{}, fmt.Errorf("%w: unsupported schedule_type", ErrInvalidCampaign)
	}
}

func recurringWindow(recurrence string, windowStart, windowEnd time.Time, untilAt *time.Time, now time.Time) (scheduleWindow, error) {
	recur := normalizeRecurrence(recurrence)
	if recur == "" {
		return scheduleWindow{}, fmt.Errorf("%w: recurrence is required", ErrInvalidCampaign)
	}
	duration := windowEnd.Sub(windowStart)
	start := windowStart
	for start.Add(duration).Before(now) || start.Add(duration).Equal(now) {
		start = addRecurrence(start, recur)
		if untilAt != nil && start.After(*untilAt) {
			return scheduleWindow{Expired: true}, nil
		}
	}
	end := start.Add(duration)
	if untilAt != nil && now.After(*untilAt) && now.After(end) {
		return scheduleWindow{Expired: true}, nil
	}
	if !now.Before(start) && now.Before(end) {
		nextStart := addRecurrence(start, recur)
		var nextEnd *time.Time
		if untilAt == nil || !nextStart.After(*untilAt) {
			value := nextStart.Add(duration)
			nextEnd = &value
		} else {
			nextStart = time.Time{}
		}
		if nextStart.IsZero() {
			return scheduleWindow{Start: start, End: end}, nil
		}
		return scheduleWindow{Start: start, End: end, NextStart: &nextStart, NextEnd: nextEnd}, nil
	}
	if untilAt != nil && start.After(*untilAt) {
		return scheduleWindow{Expired: true}, nil
	}
	return scheduleWindow{NextStart: &start, NextEnd: ptrTime(end)}, nil
}

func validateSchedule(input ScheduleInput) error {
	if input.WindowStart.IsZero() {
		return fmt.Errorf("%w: window_start is required", ErrInvalidCampaign)
	}
	if !input.WindowEnd.After(input.WindowStart) {
		return fmt.Errorf("%w: window_end must be after window_start", ErrInvalidCampaign)
	}
	if input.UntilAt != nil && input.UntilAt.Before(input.WindowStart) {
		return fmt.Errorf("%w: until_at must be on or after window_start", ErrInvalidCampaign)
	}
	switch input.ScheduleType {
	case models.DiscountScheduleTypeOneTime:
		return nil
	case models.DiscountScheduleTypeRecurring:
		switch normalizeRecurrence(input.Recurrence) {
		case models.DiscountRecurrenceDaily, models.DiscountRecurrenceWeekly, models.DiscountRecurrenceMonthly:
			return nil
		default:
			return fmt.Errorf("%w: unsupported recurrence", ErrInvalidCampaign)
		}
	default:
		return fmt.Errorf("%w: unsupported schedule_type", ErrInvalidCampaign)
	}
}

func addRecurrence(value time.Time, recurrence string) time.Time {
	switch normalizeRecurrence(recurrence) {
	case models.DiscountRecurrenceWeekly:
		return value.AddDate(0, 0, 7)
	case models.DiscountRecurrenceMonthly:
		return value.AddDate(0, 1, 0)
	default:
		return value.AddDate(0, 0, 1)
	}
}

func normalizeRecurrence(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "freq=daily", "daily":
		return models.DiscountRecurrenceDaily
	case "freq=weekly", "weekly":
		return models.DiscountRecurrenceWeekly
	case "freq=monthly", "monthly":
		return models.DiscountRecurrenceMonthly
	default:
		return trimmed
	}
}

func normalizeTimezone(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "UTC"
	}
	return trimmed
}

func sameTimePtr(left *time.Time, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.UTC().Equal(right.UTC())
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
