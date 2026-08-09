package cms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GovernanceInput struct {
	ApprovalRequired       bool
	InvalidationWebhookURL string
	Roles                  []RoleAssignmentInput
}

type RoleAssignmentInput struct {
	Subject string
	Role    string
}

type Governance struct {
	ApprovalRequired       bool
	InvalidationWebhookURL string
	Roles                  []models.CMSRoleAssignment
}

func (s *Service) Governance(ctx context.Context) (*Governance, error) {
	db := s.db.WithContext(ctx)
	var settings models.CMSSettings
	if err := db.FirstOrCreate(&settings, models.CMSSettings{ID: 1, ApprovalRequired: true}).Error; err != nil {
		return nil, err
	}
	var roles []models.CMSRoleAssignment
	if err := db.Order("subject ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return &Governance{ApprovalRequired: settings.ApprovalRequired, InvalidationWebhookURL: settings.InvalidationWebhookURL, Roles: roles}, nil
}

func (s *Service) UpdateGovernance(ctx context.Context, input GovernanceInput) (*Governance, error) {
	seen := make(map[string]struct{}, len(input.Roles))
	for i := range input.Roles {
		input.Roles[i].Subject = strings.TrimSpace(input.Roles[i].Subject)
		if input.Roles[i].Subject == "" {
			return nil, fmt.Errorf("%w: role subject is required", ErrPermissionDenied)
		}
		switch input.Roles[i].Role {
		case "author", "editor", "publisher":
		default:
			return nil, fmt.Errorf("%w: invalid CMS role", ErrPermissionDenied)
		}
		if _, ok := seen[input.Roles[i].Subject]; ok {
			return nil, fmt.Errorf("%w: duplicate role subject", ErrPermissionDenied)
		}
		seen[input.Roles[i].Subject] = struct{}{}
	}
	db := s.db.WithContext(ctx)
	if err := db.Transaction(func(tx *gorm.DB) error {
		settings := models.CMSSettings{ID: 1, ApprovalRequired: input.ApprovalRequired, InvalidationWebhookURL: strings.TrimSpace(input.InvalidationWebhookURL)}
		if err := tx.Select("id", "approval_required", "invalidation_webhook_url", "updated_at").Save(&settings).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.CMSRoleAssignment{}).Error; err != nil {
			return err
		}
		for _, role := range input.Roles {
			if err := tx.Create(&models.CMSRoleAssignment{Subject: role.Subject, Role: role.Role}).Error; err != nil {
				return err
			}
		}
		return createAuditEvent(tx, 0, nil, nil, "governance.updated", "system", "CMS governance updated")
	}); err != nil {
		return nil, err
	}
	return s.Governance(ctx)
}

type Operations struct {
	PendingSchedules  int64
	ActiveExperiments int64
	Invalidations     []models.CMSInvalidationEvent
}

func (s *Service) Operations(ctx context.Context) (*Operations, error) {
	db := s.db.WithContext(ctx)
	result := &Operations{}
	if err := db.Model(&models.CMSSchedule{}).Where("status = ?", models.CMSScheduleStatusPending).Count(&result.PendingSchedules).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.CMSExperiment{}).Where("status = ?", models.CMSExperimentStatusActive).Count(&result.ActiveExperiments).Error; err != nil {
		return nil, err
	}
	if err := db.Order("created_at DESC, id DESC").Limit(100).Find(&result.Invalidations).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) RetryInvalidation(ctx context.Context, id uint) (*models.CMSInvalidationEvent, error) {
	db := s.db.WithContext(ctx)
	result := db.Model(&models.CMSInvalidationEvent{}).Where("id = ?", id).Updates(map[string]any{"status": "pending", "last_error": "", "sent_at": nil})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	var event models.CMSInvalidationEvent
	if err := db.First(&event, id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *Service) PreviewRestore(ctx context.Context, raw []byte) (valid bool, schemaVersion, pages, navigation, globals, variants int, warnings, validationErrors []string) {
	var bundle restoreBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return false, 0, 0, 0, 0, 0, nil, []string{"malformed JSON"}
	}
	schemaVersion = bundle.SchemaVersion
	pages, navigation, globals, variants = len(bundle.Pages), len(bundle.Navigation), len(bundle.GlobalRegions), len(bundle.Variants)
	if bundle.SchemaVersion != 1 {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported schema version %d", bundle.SchemaVersion))
	}
	if err := validateRestoreBundle(bundle); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}
	localeInputs := make([]LocaleInput, 0, len(bundle.Locales))
	for _, locale := range bundle.Locales {
		localeInputs = append(localeInputs, LocaleInput{Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault, FallbackLocale: stringValue(locale.FallbackLocale)})
	}
	if err := validateLocales(localeInputs); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}
	return len(validationErrors) == 0, schemaVersion, pages, navigation, globals, variants, warnings, validationErrors
}

type EntryVariantInput struct {
	Locale, Market, ChangeSummary, Actor string
	Payload                              PagePayload
}

func (s *Service) ListEntryVariants(ctx context.Context, entryID uint) ([]models.CMSContentVariant, error) {
	db := s.db.WithContext(ctx)
	var variants []models.CMSContentVariant
	err := db.Where("entry_id = ?", entryID).Order("locale ASC, market ASC, id ASC").Find(&variants).Error
	return variants, err
}

func (s *Service) SaveEntryVariant(ctx context.Context, entryID, variantID uint, input EntryVariantInput) (*models.CMSContentVariant, error) {
	input.Locale = normalizeLocale(input.Locale)
	input.Market = strings.ToUpper(strings.TrimSpace(input.Market))
	if !localeCodePattern.MatchString(input.Locale) || (input.Market != "" && !marketCodePattern.MatchString(input.Market)) {
		return nil, ErrInvalidLocale
	}
	payload, err := prepareDraftPayload(input.Payload)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	var saved models.CMSContentVariant
	var cleanupIDs []string
	err = db.Transaction(func(tx *gorm.DB) error {
		var entry models.CMSEntry
		if err := tx.First(&entry, entryID).Error; err != nil {
			return ErrNotFound
		}
		if variantID == 0 {
			saved = models.CMSContentVariant{EntryID: entryID, Revision: 1}
		} else if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND entry_id = ?", variantID, entryID).First(&saved).Error; err != nil {
			return ErrNotFound
		} else {
			saved.Revision++
		}
		saved.Locale, saved.Market, saved.DraftPayloadJSON = input.Locale, input.Market, string(raw)
		saved.Status, saved.SubmittedBy, saved.ApprovedBy = models.CMSVariantStatusDraft, "", ""
		if saved.ID == 0 {
			if err := tx.Select("*").Create(&saved).Error; err != nil {
				if isUniqueConstraint(err) {
					return ErrDuplicateVariant
				}
				return err
			}
		} else if err := tx.Save(&saved).Error; err != nil {
			return err
		}
		cleanupIDs, err = syncPayloadMediaReferences(tx, media.OwnerTypeCMSContentVariant, saved.ID, payload, media.RoleCMSDraftContent)
		if err != nil {
			return err
		}
		return createAuditEvent(tx, entryID, nil, &saved.ID, "entry_variant.draft_saved", input.Actor, input.ChangeSummary)
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return &saved, err
}

func (s *Service) DeleteEntryVariant(ctx context.Context, entryID, variantID uint, actor string) error {
	db := s.db.WithContext(ctx)
	var cleanupIDs []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var refs []models.MediaReference
		if err := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeCMSContentVariant, variantID).Find(&refs).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND entry_id = ?", variantID, entryID).Delete(&models.CMSContentVariant{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := tx.Where("owner_type = ? AND owner_id = ?", media.OwnerTypeCMSContentVariant, variantID).Delete(&models.MediaReference{}).Error; err != nil {
			return err
		}
		for _, ref := range refs {
			cleanupIDs = append(cleanupIDs, ref.MediaID)
		}
		return createAuditEvent(tx, entryID, nil, &variantID, "entry_variant.deleted", actor, "")
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return err
}

func (s *Service) TransitionEntryVariant(ctx context.Context, entryID, variantID uint, action, actor, role, comment string) (*models.CMSContentVariant, error) {
	if (action == "approve" || action == "request_changes") && role != "editor" && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	if action == "publish" && role != "publisher" {
		return nil, ErrPermissionDenied
	}
	db := s.db.WithContext(ctx)
	var variant models.CMSContentVariant
	var cleanupIDs []string
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND entry_id = ?", variantID, entryID).First(&variant).Error; err != nil {
			return ErrNotFound
		}
		now := time.Now().UTC()
		switch action {
		case "submit":
			if variant.Status != models.CMSVariantStatusDraft && variant.Status != models.CMSVariantStatusChangesRequested {
				return ErrInvalidTransition
			}
			variant.Status, variant.SubmittedBy = models.CMSVariantStatusInReview, actor
		case "approve":
			if variant.Status != models.CMSVariantStatusInReview {
				return ErrInvalidTransition
			}
			variant.Status, variant.ApprovedBy = models.CMSVariantStatusApproved, actor
		case "request_changes":
			if variant.Status != models.CMSVariantStatusInReview {
				return ErrInvalidTransition
			}
			variant.Status = models.CMSVariantStatusChangesRequested
		case "reset":
			variant.Status, variant.SubmittedBy, variant.ApprovedBy = models.CMSVariantStatusDraft, "", ""
		case "publish":
			if variant.Status != models.CMSVariantStatusApproved {
				return ErrApprovalRequired
			}
			payload, err := publicationPayloadJSON(tx, variant.DraftPayloadJSON)
			if err != nil {
				return err
			}
			variant.Status, variant.PublishedPayloadJSON, variant.PublishedAt = models.CMSVariantStatusPublished, variant.DraftPayloadJSON, &now
			cleanupIDs, err = syncPayloadMediaReferences(tx, media.OwnerTypeCMSContentVariant, variant.ID, payload, media.RoleCMSContent)
			if err != nil {
				return err
			}
			if err := createInvalidationEvent(tx, entryID, &variant.ID, "entry_variant.published"); err != nil {
				return err
			}
		default:
			return ErrInvalidTransition
		}
		if err := tx.Save(&variant).Error; err != nil {
			return err
		}
		if strings.TrimSpace(comment) != "" {
			if err := tx.Create(&models.CMSChangeComment{EntryID: entryID, VariantID: &variant.ID, Actor: actor, Body: strings.TrimSpace(comment), CreatedAt: now}).Error; err != nil {
				return err
			}
		}
		return createAuditEvent(tx, entryID, nil, &variant.ID, "entry_variant."+action, actor, strings.TrimSpace(comment))
	})
	if err == nil {
		s.cleanupOrphanMedia(cleanupIDs)
	}
	return &variant, err
}

func (s *Service) CreateComment(ctx context.Context, entryID uint, actor, body string) (*models.CMSChangeComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("%w: comment body is required", ErrInvalidPage)
	}
	db := s.db.WithContext(ctx)
	var count int64
	if err := db.Model(&models.CMSEntry{}).Where("id = ?", entryID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrNotFound
	}
	comment := models.CMSChangeComment{EntryID: entryID, Actor: actor, Body: body, CreatedAt: time.Now().UTC()}
	if err := db.Create(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *Service) ResolveComment(ctx context.Context, id uint, actor string) (*models.CMSChangeComment, error) {
	db := s.db.WithContext(ctx)
	var comment models.CMSChangeComment
	if err := db.First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	comment.ResolvedAt, comment.ResolvedBy = &now, actor
	if err := db.Save(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *Service) EntryWorkflow(ctx context.Context, entryID uint) (*models.CMSEntryWorkflow, []models.CMSChangeComment, error) {
	db := s.db.WithContext(ctx)
	var entry models.CMSEntry
	if err := db.First(&entry, entryID).Error; err != nil {
		return nil, nil, ErrNotFound
	}
	if entry.CurrentVersionID == nil {
		return nil, nil, ErrNoDraft
	}
	var workflow models.CMSEntryWorkflow
	err := db.Where("entry_id = ?", entryID).First(&workflow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		workflow = models.CMSEntryWorkflow{EntryID: entryID, VersionID: *entry.CurrentVersionID, Status: models.CMSWorkflowStatusDraft}
		if err := db.Select("*").Create(&workflow).Error; err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}
	var comments []models.CMSChangeComment
	if err := db.Where("entry_id = ?", entryID).Order("created_at ASC, id ASC").Find(&comments).Error; err != nil {
		return nil, nil, err
	}
	return &workflow, comments, nil
}

func (s *Service) TransitionEntryWorkflow(ctx context.Context, entryID uint, action, actor, role, comment string) (*models.CMSEntryWorkflow, []models.CMSChangeComment, error) {
	workflow, _, err := s.EntryWorkflow(ctx, entryID)
	if err != nil {
		return nil, nil, err
	}
	if (action == "approve" || action == "request_changes") && role != "editor" && role != "publisher" {
		return nil, nil, ErrPermissionDenied
	}
	db := s.db.WithContext(ctx)
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(workflow, workflow.ID).Error; err != nil {
			return err
		}
		switch action {
		case "submit":
			if workflow.Status != models.CMSWorkflowStatusDraft && workflow.Status != models.CMSWorkflowStatusChangesRequested {
				return ErrInvalidTransition
			}
			workflow.Status, workflow.SubmittedBy = models.CMSWorkflowStatusInReview, actor
		case "approve":
			if workflow.Status != models.CMSWorkflowStatusInReview {
				return ErrInvalidTransition
			}
			workflow.Status, workflow.ApprovedBy = models.CMSWorkflowStatusApproved, actor
		case "request_changes":
			if workflow.Status != models.CMSWorkflowStatusInReview {
				return ErrInvalidTransition
			}
			workflow.Status = models.CMSWorkflowStatusChangesRequested
		case "reset":
			workflow.Status, workflow.SubmittedBy, workflow.ApprovedBy = models.CMSWorkflowStatusDraft, "", ""
		default:
			return ErrInvalidTransition
		}
		if err := tx.Save(workflow).Error; err != nil {
			return err
		}
		if strings.TrimSpace(comment) != "" {
			if err := tx.Create(&models.CMSChangeComment{EntryID: entryID, Actor: actor, Body: strings.TrimSpace(comment), CreatedAt: time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		return createAuditEvent(tx, entryID, &workflow.VersionID, nil, "workflow."+action, actor, strings.TrimSpace(comment))
	})
	if err != nil {
		return nil, nil, err
	}
	return s.EntryWorkflow(ctx, entryID)
}
