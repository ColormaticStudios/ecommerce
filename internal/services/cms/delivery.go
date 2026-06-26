package cms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidDelivery = errors.New("invalid cms delivery configuration")

type TargetingRule struct {
	Markets       []string `json:"markets"`
	DeviceClasses []string `json:"device_classes"`
	AuthStates    []string `json:"auth_states"`
	Referrers     []string `json:"referrers"`
	UTMSources    []string `json:"utm_sources"`
	SegmentKeys   []string `json:"segment_keys"`
}

type TargetingRuleInput struct {
	TargetingRule
	Priority  int
	IsEnabled bool
}

type ScheduleInput struct {
	PublishAt   time.Time
	UnpublishAt *time.Time
	Timezone    string
}

type ExperimentVariantInput struct {
	Name       string
	VersionID  uint
	Allocation int
}

type ExperimentInput struct {
	Name      string
	Status    models.CMSExperimentStatus
	StickyKey string
	StartsAt  time.Time
	EndsAt    *time.Time
	Variants  []ExperimentVariantInput
}

type DeliveryInput struct {
	Schedule       *ScheduleInput
	TargetingRules []TargetingRuleInput
	Experiment     *ExperimentInput
}

type TargetingRuleRecord struct {
	Model models.CMSTargetingRule
	Rule  TargetingRule
}

type DeliveryRecord struct {
	Schedule           *models.CMSSchedule
	TargetingRules     []TargetingRuleRecord
	Experiment         *models.CMSExperiment
	RecentPublications []models.CMSPublication
}

type RequestContext struct {
	Market        string
	DeviceClass   string
	Authenticated bool
	Referrer      string
	UTMSource     string
	SegmentKey    string
	AssignmentKey string
	CustomerKey   string
	CorrelationID string
}

type DeliveryDecision struct {
	ContentVersionID    uint
	ExperimentID        *uint
	ExperimentVariantID *uint
	CorrelationID       string
	AssignmentHash      string
}

type ContentEventInput struct {
	ContentVersionID    uint
	ExperimentID        *uint
	ExperimentVariantID *uint
	CorrelationID       string
	EventType           string
}

func (s *Service) GetDelivery(pageID uint) (*DeliveryRecord, error) {
	_, entry, err := loadPageEntry(s.db, pageID, clause.Locking{})
	if err != nil {
		return nil, err
	}
	return loadDeliveryRecord(s.db, entry.ID)
}

func (s *Service) UpdateDelivery(pageID uint, input DeliveryInput) (*DeliveryRecord, error) {
	var entryID uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		_, entry, err := loadPageEntry(tx, pageID, clause.Locking{Strength: "UPDATE"})
		if err != nil {
			return err
		}
		if entry.CurrentVersionID == nil {
			return fmt.Errorf("%w: page has no content version", ErrInvalidDelivery)
		}
		entryID = entry.ID
		if err := replaceSchedule(tx, entry, input.Schedule); err != nil {
			return err
		}
		if input.Schedule != nil {
			if err := tx.Model(&models.CMSEntry{}).Where("id = ?", entry.ID).Update("status", models.CMSEntryStatusScheduled).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&models.CMSEntry{}).Where("id = ?", entry.ID).Update("status", draftStatusFor(entry)).Error; err != nil {
				return err
			}
		}
		if err := replaceTargetingRules(tx, entry, input.TargetingRules); err != nil {
			return err
		}
		if err := replaceExperiment(tx, entry, input.Experiment); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loadDeliveryRecord(s.db, entryID)
}

func replaceSchedule(tx *gorm.DB, entry models.CMSEntry, input *ScheduleInput) error {
	var existing models.CMSSchedule
	err := tx.Unscoped().Where("entry_id = ?", entry.ID).First(&existing).Error
	if input == nil {
		if err == nil {
			return tx.Unscoped().Delete(&existing).Error
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if input.PublishAt.IsZero() {
		return fmt.Errorf("%w: publish time is required", ErrInvalidDelivery)
	}
	if input.UnpublishAt != nil && !input.UnpublishAt.After(input.PublishAt) {
		return fmt.Errorf("%w: unpublish time must be after publish time", ErrInvalidDelivery)
	}
	if strings.TrimSpace(input.Timezone) == "" {
		input.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalidDelivery)
	}
	schedule := models.CMSSchedule{
		EntryID:     entry.ID,
		VersionID:   *entry.CurrentVersionID,
		PublishAt:   input.PublishAt.UTC(),
		UnpublishAt: utcTimePointer(input.UnpublishAt),
		Timezone:    input.Timezone,
		Status:      models.CMSScheduleStatusPending,
	}
	if err == nil {
		schedule.ID = existing.ID
		schedule.CreatedAt = existing.CreatedAt
		return tx.Unscoped().Save(&schedule).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&schedule).Error
}

func replaceTargetingRules(tx *gorm.DB, entry models.CMSEntry, inputs []TargetingRuleInput) error {
	if err := tx.Unscoped().Where("entry_id = ?", entry.ID).Delete(&models.CMSTargetingRule{}).Error; err != nil {
		return err
	}
	for _, input := range inputs {
		if err := validateTargetingRule(input.TargetingRule); err != nil {
			return err
		}
		raw, err := json.Marshal(normalizeRule(input.TargetingRule))
		if err != nil {
			return err
		}
		model := models.CMSTargetingRule{
			EntryID: entry.ID, VersionID: *entry.CurrentVersionID, RuleJSON: string(raw),
			Priority: input.Priority, IsEnabled: input.IsEnabled,
		}
		if err := tx.Select("*").Create(&model).Error; err != nil {
			return err
		}
	}
	return nil
}

func replaceExperiment(tx *gorm.DB, entry models.CMSEntry, input *ExperimentInput) error {
	var existing []models.CMSExperiment
	if err := tx.Where("entry_id = ?", entry.ID).Find(&existing).Error; err != nil {
		return err
	}
	for _, experiment := range existing {
		if err := tx.Where("experiment_id = ?", experiment.ID).Delete(&models.CMSExperimentVariant{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("entry_id = ?", entry.ID).Delete(&models.CMSExperiment{}).Error; err != nil {
		return err
	}
	if input == nil {
		return nil
	}
	if err := validateExperiment(tx, entry, input); err != nil {
		return err
	}
	experiment := models.CMSExperiment{
		EntryID: entry.ID, Name: strings.TrimSpace(input.Name), Status: input.Status,
		StickyKey: input.StickyKey, StartsAt: input.StartsAt.UTC(), EndsAt: utcTimePointer(input.EndsAt),
	}
	if err := tx.Create(&experiment).Error; err != nil {
		return err
	}
	for _, inputVariant := range input.Variants {
		variant := models.CMSExperimentVariant{
			ExperimentID: experiment.ID, Name: strings.TrimSpace(inputVariant.Name),
			VersionID: inputVariant.VersionID, Allocation: inputVariant.Allocation,
		}
		if err := tx.Create(&variant).Error; err != nil {
			return err
		}
	}
	return nil
}

func validateExperiment(tx *gorm.DB, entry models.CMSEntry, input *ExperimentInput) error {
	if strings.TrimSpace(input.Name) == "" || input.StartsAt.IsZero() {
		return fmt.Errorf("%w: experiment name and start time are required", ErrInvalidDelivery)
	}
	switch input.Status {
	case models.CMSExperimentStatusDraft, models.CMSExperimentStatusActive, models.CMSExperimentStatusPaused, models.CMSExperimentStatusCompleted:
	default:
		return fmt.Errorf("%w: invalid experiment status", ErrInvalidDelivery)
	}
	if input.StickyKey != "visitor" && input.StickyKey != "customer" {
		return fmt.Errorf("%w: invalid sticky assignment key", ErrInvalidDelivery)
	}
	if input.EndsAt != nil && !input.EndsAt.After(input.StartsAt) {
		return fmt.Errorf("%w: experiment end must be after start", ErrInvalidDelivery)
	}
	if len(input.Variants) < 2 {
		return fmt.Errorf("%w: experiments require at least two variants", ErrInvalidDelivery)
	}
	total := 0
	names := map[string]bool{}
	for _, variant := range input.Variants {
		name := strings.ToLower(strings.TrimSpace(variant.Name))
		if name == "" || names[name] || variant.Allocation < 1 {
			return fmt.Errorf("%w: variants require unique names and positive allocation", ErrInvalidDelivery)
		}
		names[name] = true
		total += variant.Allocation
		var count int64
		if err := tx.Model(&models.CMSEntryVersion{}).Where("id = ? AND entry_id = ?", variant.VersionID, entry.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("%w: experiment variant version does not belong to this page", ErrInvalidDelivery)
		}
	}
	if total != 10000 {
		return fmt.Errorf("%w: variant allocation must total 100 percent", ErrInvalidDelivery)
	}
	return nil
}

func loadDeliveryRecord(db *gorm.DB, entryID uint) (*DeliveryRecord, error) {
	record := &DeliveryRecord{TargetingRules: []TargetingRuleRecord{}, RecentPublications: []models.CMSPublication{}}
	var schedule models.CMSSchedule
	if err := db.Where("entry_id = ?", entryID).First(&schedule).Error; err == nil {
		record.Schedule = &schedule
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var rules []models.CMSTargetingRule
	if err := db.Where("entry_id = ?", entryID).Order("priority ASC, id ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, model := range rules {
		var rule TargetingRule
		if err := json.Unmarshal([]byte(model.RuleJSON), &rule); err != nil {
			return nil, err
		}
		record.TargetingRules = append(record.TargetingRules, TargetingRuleRecord{Model: model, Rule: rule})
	}
	var experiment models.CMSExperiment
	if err := db.Where("entry_id = ?", entryID).Order("created_at DESC, id DESC").First(&experiment).Error; err == nil {
		if err := db.Where("experiment_id = ?", experiment.ID).Order("id ASC").Find(&experiment.Variants).Error; err != nil {
			return nil, err
		}
		record.Experiment = &experiment
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := db.Where("entry_id = ?", entryID).Order("published_at DESC, id DESC").Limit(20).Find(&record.RecentPublications).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Service) ResolveDelivery(record *PageRecord, request RequestContext, now time.Time) (*DeliveryDecision, bool, error) {
	if record.PublishedVersion == nil {
		return nil, false, ErrNotFound
	}
	delivery, err := loadDeliveryRecord(s.db, record.Entry.ID)
	if err != nil {
		return nil, false, err
	}
	if !matchesTargeting(delivery.TargetingRules, record.PublishedVersion.ID, request) {
		return nil, false, nil
	}
	decision := &DeliveryDecision{ContentVersionID: record.PublishedVersion.ID, CorrelationID: request.CorrelationID}
	if delivery.Experiment != nil && experimentActive(*delivery.Experiment, now.UTC()) {
		stickyKey := request.AssignmentKey
		if delivery.Experiment.StickyKey == "customer" && request.CustomerKey != "" {
			stickyKey = request.CustomerKey
		}
		variant := allocateVariant(*delivery.Experiment, stickyKey)
		if variant != nil {
			decision.ContentVersionID = variant.VersionID
			decision.ExperimentID = &delivery.Experiment.ID
			decision.ExperimentVariantID = &variant.ID
			decision.AssignmentHash = assignmentHash(delivery.Experiment.ID, stickyKey)
		}
	}
	return decision, true, nil
}

func (s *Service) LoadVersion(entryID, versionID uint) (*models.CMSEntryVersion, error) {
	var version models.CMSEntryVersion
	if err := s.db.Where("id = ? AND entry_id = ?", versionID, entryID).First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &version, nil
}

func matchesTargeting(records []TargetingRuleRecord, versionID uint, request RequestContext) bool {
	hasRule := false
	for _, record := range records {
		if !record.Model.IsEnabled || record.Model.VersionID != versionID {
			continue
		}
		hasRule = true
		if ruleMatches(record.Rule, request) {
			return true
		}
	}
	return !hasRule
}

func ruleMatches(rule TargetingRule, request RequestContext) bool {
	authState := "guest"
	if request.Authenticated {
		authState = "authenticated"
	}
	return matchesValue(rule.Markets, request.Market) &&
		matchesValue(rule.DeviceClasses, request.DeviceClass) &&
		matchesValue(rule.AuthStates, authState) &&
		matchesReferrer(rule.Referrers, request.Referrer) &&
		matchesValue(rule.UTMSources, request.UTMSource) &&
		matchesValue(rule.SegmentKeys, request.SegmentKey)
}

func matchesValue(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	actual = strings.ToLower(strings.TrimSpace(actual))
	for _, value := range allowed {
		if strings.ToLower(strings.TrimSpace(value)) == actual {
			return true
		}
	}
	return false
}

func matchesReferrer(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	actual = strings.ToLower(strings.TrimSpace(actual))
	for _, value := range allowed {
		if strings.Contains(actual, strings.ToLower(strings.TrimSpace(value))) {
			return true
		}
	}
	return false
}

func allocateVariant(experiment models.CMSExperiment, key string) *models.CMSExperimentVariant {
	if key == "" || len(experiment.Variants) == 0 {
		return nil
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", experiment.ID, key)))
	bucket := int(uint16(digest[0])<<8|uint16(digest[1])) * 10000 / 65536
	cumulative := 0
	for index := range experiment.Variants {
		cumulative += experiment.Variants[index].Allocation
		if bucket < cumulative {
			return &experiment.Variants[index]
		}
	}
	return &experiment.Variants[len(experiment.Variants)-1]
}

func assignmentHash(experimentID uint, key string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", experimentID, key)))
	return hex.EncodeToString(digest[:])
}

func experimentActive(experiment models.CMSExperiment, now time.Time) bool {
	return experiment.Status == models.CMSExperimentStatusActive && !now.Before(experiment.StartsAt) &&
		(experiment.EndsAt == nil || now.Before(*experiment.EndsAt))
}

func (s *Service) RecordContentEvent(input ContentEventInput) error {
	if input.ContentVersionID == 0 || strings.TrimSpace(input.CorrelationID) == "" ||
		(input.EventType != "impression" && input.EventType != "conversion") {
		return fmt.Errorf("%w: invalid content event", ErrInvalidDelivery)
	}
	var version models.CMSEntryVersion
	if err := s.db.First(&version, input.ContentVersionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	event := models.CMSExposureEvent{
		EntryID: version.EntryID, ContentVersionID: version.ID, ExperimentID: input.ExperimentID,
		ExperimentVariantID: input.ExperimentVariantID, CorrelationID: strings.TrimSpace(input.CorrelationID),
		EventType: input.EventType, CreatedAt: time.Now().UTC(),
	}
	if input.ExperimentID != nil {
		if input.ExperimentVariantID == nil {
			return fmt.Errorf("%w: experiment variant is required", ErrInvalidDelivery)
		}
		var count int64
		if err := s.db.Model(&models.CMSExperimentVariant{}).
			Where("id = ? AND experiment_id = ? AND version_id = ?", *input.ExperimentVariantID, *input.ExperimentID, input.ContentVersionID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("%w: event does not match an experiment variant", ErrInvalidDelivery)
		}
	}
	err := s.db.Create(&event).Error
	if err == nil {
		return nil
	}
	if isUniqueConstraint(err) {
		return nil
	}
	return err
}

type ReconcileSummary struct {
	Published            int
	Unpublished          int
	CompletedExperiments int
}

func ReconcileDelivery(db *gorm.DB, now time.Time, mediaServices ...*media.Service) (ReconcileSummary, error) {
	summary := ReconcileSummary{}
	service := NewPageService(db, mediaServices...)
	var schedules []models.CMSSchedule
	if err := db.Where("status IN ?", []models.CMSScheduleStatus{models.CMSScheduleStatusPending, models.CMSScheduleStatusActive}).Find(&schedules).Error; err != nil {
		return summary, err
	}
	for _, candidate := range schedules {
		var cleanupIDs []string
		err := db.Transaction(func(tx *gorm.DB) error {
			var schedule models.CMSSchedule
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&schedule, candidate.ID).Error; err != nil {
				return err
			}
			var entry models.CMSEntry
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&entry, schedule.EntryID).Error; err != nil {
				return err
			}
			transitionedAt := now.UTC()
			if schedule.Status == models.CMSScheduleStatusPending && !now.Before(schedule.PublishAt) {
				publication := models.CMSPublication{EntryID: entry.ID, VersionID: schedule.VersionID, PublishedAt: transitionedAt, Notes: "Scheduled publication"}
				if err := tx.Create(&publication).Error; err != nil {
					return err
				}
				entry.PublishedVersionID = &schedule.VersionID
				entry.Status = models.CMSEntryStatusPublished
				schedule.Status = models.CMSScheduleStatusActive
				schedule.LastTransitionAt = &transitionedAt
				summary.Published++
				var version models.CMSEntryVersion
				if err := tx.Where("id = ? AND entry_id = ?", schedule.VersionID, entry.ID).First(&version).Error; err != nil {
					return err
				}
				payload, err := payloadFromVersion(version)
				if err != nil {
					return err
				}
				cleanupIDs, err = syncContentMediaReferences(tx, entry.ID, payload, media.RoleCMSContent)
				if err != nil {
					return err
				}
			}
			if schedule.UnpublishAt != nil && !now.Before(*schedule.UnpublishAt) && schedule.Status == models.CMSScheduleStatusActive {
				entry.PublishedVersionID = nil
				entry.Status = models.CMSEntryStatusArchived
				schedule.Status = models.CMSScheduleStatusCompleted
				schedule.LastTransitionAt = &transitionedAt
				summary.Unpublished++
				var liveRefs []models.MediaReference
				if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeCMSEntry, entry.ID, media.RoleCMSContent).Find(&liveRefs).Error; err != nil {
					return err
				}
				if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeCMSEntry, entry.ID, media.RoleCMSContent).Delete(&models.MediaReference{}).Error; err != nil {
					return err
				}
				for _, ref := range liveRefs {
					cleanupIDs = append(cleanupIDs, ref.MediaID)
				}
			}
			if err := tx.Save(&entry).Error; err != nil {
				return err
			}
			return tx.Save(&schedule).Error
		})
		if err != nil {
			return summary, err
		}
		service.cleanupOrphanMedia(cleanupIDs)
	}
	result := db.Model(&models.CMSExperiment{}).
		Where("status IN ? AND ends_at IS NOT NULL AND ends_at <= ?", []models.CMSExperimentStatus{models.CMSExperimentStatusActive, models.CMSExperimentStatusPaused}, now.UTC()).
		Update("status", models.CMSExperimentStatusCompleted)
	if result.Error != nil {
		return summary, result.Error
	}
	summary.CompletedExperiments = int(result.RowsAffected)
	return summary, nil
}

func StartDeliveryWorker(ctx context.Context, db *gorm.DB, interval time.Duration, logger *log.Logger, mediaServices ...*media.Service) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if summary, err := ReconcileDelivery(db, time.Now().UTC(), mediaServices...); err != nil {
				logger.Printf("[ERROR] CMS delivery reconciliation failed: %v", err)
			} else if summary.Published > 0 || summary.Unpublished > 0 || summary.CompletedExperiments > 0 {
				logger.Printf("[INFO] CMS delivery reconciliation published=%d unpublished=%d completed_experiments=%d", summary.Published, summary.Unpublished, summary.CompletedExperiments)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func validateTargetingRule(rule TargetingRule) error {
	validDevices := map[string]bool{"desktop": true, "mobile": true, "tablet": true}
	for _, value := range rule.DeviceClasses {
		if !validDevices[strings.ToLower(strings.TrimSpace(value))] {
			return fmt.Errorf("%w: invalid device class", ErrInvalidDelivery)
		}
	}
	validAuth := map[string]bool{"guest": true, "authenticated": true}
	for _, value := range rule.AuthStates {
		if !validAuth[strings.ToLower(strings.TrimSpace(value))] {
			return fmt.Errorf("%w: invalid authentication state", ErrInvalidDelivery)
		}
	}
	return nil
}

func normalizeRule(rule TargetingRule) TargetingRule {
	rule.Markets = normalizeList(rule.Markets)
	rule.DeviceClasses = normalizeList(rule.DeviceClasses)
	rule.AuthStates = normalizeList(rule.AuthStates)
	rule.Referrers = normalizeList(rule.Referrers)
	rule.UTMSources = normalizeList(rule.UTMSources)
	rule.SegmentKeys = normalizeList(rule.SegmentKeys)
	return rule
}

func normalizeList(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
