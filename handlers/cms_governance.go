package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type cmsGovernanceInput struct {
	ApprovalRequired       bool   `json:"approval_required"`
	InvalidationWebhookURL string `json:"invalidation_webhook_url"`
	Roles                  []struct {
		Subject string `json:"subject"`
		Role    string `json:"role"`
	} `json:"roles"`
}

func cmsGovernanceResponse(db *gorm.DB, c *gin.Context) {
	var settings models.CMSSettings
	if err := db.FirstOrCreate(&settings, models.CMSSettings{ID: 1, ApprovalRequired: true}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load CMS governance"})
		return
	}
	var roles []models.CMSRoleAssignment
	if err := db.Order("subject ASC").Find(&roles).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load CMS roles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"approval_required": settings.ApprovalRequired, "invalidation_webhook_url": settings.InvalidationWebhookURL, "roles": roles})
}

func getAdminCMSGovernance(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) { cmsGovernanceResponse(db, c) }
}

func updateAdminCMSGovernance(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req cmsGovernanceInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		webhook := strings.TrimSpace(req.InvalidationWebhookURL)
		if webhook != "" {
			parsed, err := url.ParseRequestURI(webhook)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				c.JSON(400, gin.H{"error": "Invalid invalidation webhook URL"})
				return
			}
		}
		seen := map[string]bool{}
		for _, assignment := range req.Roles {
			subject := strings.TrimSpace(assignment.Subject)
			if subject == "" || seen[subject] || (assignment.Role != "author" && assignment.Role != "editor" && assignment.Role != "publisher") {
				c.JSON(400, gin.H{"error": "Roles require unique subjects and a valid CMS role"})
				return
			}
			seen[subject] = true
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&models.CMSSettings{ID: 1, ApprovalRequired: req.ApprovalRequired, InvalidationWebhookURL: webhook}).Error; err != nil {
				return err
			}
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CMSRoleAssignment{}).Error; err != nil {
				return err
			}
			for _, assignment := range req.Roles {
				if err := tx.Create(&models.CMSRoleAssignment{Subject: strings.TrimSpace(assignment.Subject), Role: assignment.Role}).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to update CMS governance"})
			return
		}
		cmsGovernanceResponse(db, c)
	}
}

func loadEntryWorkflow(db *gorm.DB, entryID uint) (models.CMSEntryWorkflow, []models.CMSChangeComment, error) {
	var entry models.CMSEntry
	if err := db.First(&entry, entryID).Error; err != nil {
		return models.CMSEntryWorkflow{}, nil, err
	}
	versionID := uint(0)
	if entry.CurrentVersionID != nil {
		versionID = *entry.CurrentVersionID
	}
	workflow := models.CMSEntryWorkflow{EntryID: entryID, VersionID: versionID, Status: models.CMSWorkflowStatusDraft}
	if err := db.Where("entry_id = ?", entryID).FirstOrCreate(&workflow).Error; err != nil {
		return workflow, nil, err
	}
	if workflow.VersionID != versionID {
		workflow.VersionID = versionID
		workflow.Status = models.CMSWorkflowStatusDraft
		workflow.SubmittedBy = ""
		workflow.ApprovedBy = ""
		if err := db.Save(&workflow).Error; err != nil {
			return workflow, nil, err
		}
	}
	var comments []models.CMSChangeComment
	err := db.Where("entry_id = ?", entryID).Order("created_at DESC").Find(&comments).Error
	return workflow, comments, err
}

func workflowJSON(workflow models.CMSEntryWorkflow, comments []models.CMSChangeComment) gin.H {
	return gin.H{"entry_id": workflow.EntryID, "version_id": workflow.VersionID, "status": workflow.Status, "submitted_by": nullableText(workflow.SubmittedBy), "approved_by": nullableText(workflow.ApprovedBy), "comments": comments}
}
func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func getAdminCMSEntryWorkflow(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid entry id"})
			return
		}
		workflow, comments, err := loadEntryWorkflow(db, id)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(200, workflowJSON(workflow, comments))
	}
}

func transitionAdminCMSEntryWorkflow(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid entry id"})
			return
		}
		workflow, comments, err := loadEntryWorkflow(db, entryID)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		role, err := cmsRoleForSubject(db, c.GetString("userID"))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		switch c.Param("action") {
		case "submit":
			if workflow.Status != models.CMSWorkflowStatusDraft && workflow.Status != models.CMSWorkflowStatusChangesRequested {
				c.JSON(400, gin.H{"error": "Only drafts can be submitted"})
				return
			}
			workflow.Status = models.CMSWorkflowStatusInReview
			workflow.SubmittedBy = c.GetString("userID")
		case "approve":
			if role != "publisher" || workflow.Status != models.CMSWorkflowStatusInReview {
				c.JSON(403, gin.H{"error": "Publisher approval is required"})
				return
			}
			workflow.Status = models.CMSWorkflowStatusApproved
			workflow.ApprovedBy = c.GetString("userID")
		case "request_changes":
			if role == "author" || workflow.Status != models.CMSWorkflowStatusInReview {
				c.JSON(403, gin.H{"error": "Editor permission is required"})
				return
			}
			workflow.Status = models.CMSWorkflowStatusChangesRequested
			workflow.ApprovedBy = ""
		case "reset":
			workflow.Status = models.CMSWorkflowStatusDraft
			workflow.SubmittedBy = ""
			workflow.ApprovedBy = ""
		default:
			c.JSON(400, gin.H{"error": "Invalid workflow action"})
			return
		}
		if err := db.Save(&workflow).Error; err != nil {
			c.JSON(500, gin.H{"error": "Failed to update workflow"})
			return
		}
		c.JSON(200, workflowJSON(workflow, comments))
	}
}

func cmsRoleForSubject(db *gorm.DB, subject string) (string, error) {
	var row models.CMSRoleAssignment
	err := db.Where("subject = ?", subject).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return "publisher", nil
	}
	return row.Role, err
}

func createAdminCMSEntryComment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid entry id"})
			return
		}
		var req struct {
			Body string `json:"body"`
		}
		if err := bindStrictJSON(c, &req); err != nil || strings.TrimSpace(req.Body) == "" {
			c.JSON(400, gin.H{"error": "Comment is required"})
			return
		}
		comment := models.CMSChangeComment{EntryID: entryID, Actor: c.GetString("userID"), Body: strings.TrimSpace(req.Body), CreatedAt: time.Now().UTC()}
		if err := db.Create(&comment).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(201, comment)
	}
}
func resolveAdminCMSComment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid comment id"})
			return
		}
		var comment models.CMSChangeComment
		if err := db.First(&comment, id).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		now := time.Now().UTC()
		comment.ResolvedAt = &now
		comment.ResolvedBy = c.GetString("userID")
		if err := db.Save(&comment).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(200, comment)
	}
}

type cmsEntryVariantInput struct {
	Locale        string         `json:"locale"`
	Market        string         `json:"market"`
	Payload       map[string]any `json:"payload"`
	ChangeSummary string         `json:"change_summary"`
}

func variantJSON(v models.CMSContentVariant) gin.H {
	var payload map[string]any
	raw := v.DraftPayloadJSON
	if v.Status == models.CMSVariantStatusPublished {
		raw = v.PublishedPayloadJSON
	}
	_ = json.Unmarshal([]byte(raw), &payload)
	return gin.H{"id": v.ID, "entry_id": v.EntryID, "locale": v.Locale, "market": v.Market, "payload": payload, "status": v.Status, "revision": v.Revision, "submitted_by": nullableText(v.SubmittedBy), "approved_by": nullableText(v.ApprovedBy), "published_at": v.PublishedAt, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}
func listAdminCMSEntryVariants(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid entry id"})
			return
		}
		var rows []models.CMSContentVariant
		if err := db.Where("entry_id = ?", id).Order("locale, market").Find(&rows).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, v := range rows {
			out = append(out, variantJSON(v))
		}
		c.JSON(200, out)
	}
}
func saveAdminCMSEntryVariant(db *gorm.DB, update bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid entry id"})
			return
		}
		var req cmsEntryVariantInput
		if err := bindStrictJSON(c, &req); err != nil || strings.TrimSpace(req.Locale) == "" || req.Payload == nil {
			c.JSON(400, gin.H{"error": "Locale and payload are required"})
			return
		}
		raw, _ := json.Marshal(req.Payload)
		v := models.CMSContentVariant{EntryID: entryID, Locale: strings.TrimSpace(req.Locale), Market: strings.TrimSpace(req.Market), DraftPayloadJSON: string(raw), Status: models.CMSVariantStatusDraft, Revision: 1}
		if update {
			variantID, e := parsePositivePathID(c, "variant_id")
			if e != nil || db.Where("entry_id = ?", entryID).First(&v, variantID).Error != nil {
				c.JSON(404, gin.H{"error": "Variant not found"})
				return
			}
			v.Locale = strings.TrimSpace(req.Locale)
			v.Market = strings.TrimSpace(req.Market)
			v.DraftPayloadJSON = string(raw)
			v.Status = models.CMSVariantStatusDraft
			v.Revision++
		}
		if err := db.Save(&v).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		status := 201
		if update {
			status = 200
		}
		c.JSON(status, variantJSON(v))
	}
}
func createAdminCMSEntryVariant(db *gorm.DB) gin.HandlerFunc {
	return saveAdminCMSEntryVariant(db, false)
}
func updateAdminCMSEntryVariant(db *gorm.DB) gin.HandlerFunc {
	return saveAdminCMSEntryVariant(db, true)
}
func deleteAdminCMSEntryVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, e1 := parsePositivePathID(c, "id")
		id, e2 := parsePositivePathID(c, "variant_id")
		if e1 != nil || e2 != nil {
			c.JSON(400, gin.H{"error": "invalid variant id"})
			return
		}
		result := db.Where("entry_id = ?", entryID).Delete(&models.CMSContentVariant{}, id)
		if result.Error != nil {
			writeCMSError(c, result.Error)
			return
		}
		c.JSON(200, gin.H{"message": "Variant deleted"})
	}
}
func transitionAdminCMSEntryVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, e1 := parsePositivePathID(c, "id")
		id, e2 := parsePositivePathID(c, "variant_id")
		if e1 != nil || e2 != nil {
			c.JSON(400, gin.H{"error": "invalid variant id"})
			return
		}
		var v models.CMSContentVariant
		if err := db.Where("entry_id = ?", entryID).First(&v, id).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		role, _ := cmsRoleForSubject(db, c.GetString("userID"))
		switch c.Param("action") {
		case "submit":
			v.Status = models.CMSVariantStatusInReview
			v.SubmittedBy = c.GetString("userID")
		case "approve":
			if role != "publisher" {
				c.JSON(403, gin.H{"error": "Publisher permission is required"})
				return
			}
			v.Status = models.CMSVariantStatusApproved
			v.ApprovedBy = c.GetString("userID")
		case "request_changes":
			if role == "author" {
				c.JSON(403, gin.H{"error": "Editor permission is required"})
				return
			}
			v.Status = models.CMSVariantStatusChangesRequested
		case "publish":
			if role != "publisher" || v.Status != models.CMSVariantStatusApproved {
				c.JSON(403, gin.H{"error": "Approved content and publisher permission are required"})
				return
			}
			now := time.Now().UTC()
			v.Status = models.CMSVariantStatusPublished
			v.PublishedPayloadJSON = v.DraftPayloadJSON
			v.PublishedAt = &now
		case "reset":
			v.Status = models.CMSVariantStatusDraft
		default:
			c.JSON(400, gin.H{"error": "Invalid workflow action"})
			return
		}
		if err := db.Save(&v).Error; err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(200, variantJSON(v))
	}
}

func getAdminCMSOperations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var schedules, experiments int64
		db.Model(&models.CMSSchedule{}).Where("status = ?", models.CMSScheduleStatusPending).Count(&schedules)
		db.Model(&models.CMSExperiment{}).Where("status = ?", models.CMSExperimentStatusActive).Count(&experiments)
		var events []models.CMSInvalidationEvent
		db.Order("created_at DESC").Limit(100).Find(&events)
		c.JSON(200, gin.H{"pending_schedules": schedules, "active_experiments": experiments, "invalidations": events})
	}
}
func retryAdminCMSInvalidation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid event id"})
			return
		}
		result := db.Model(&models.CMSInvalidationEvent{}).Where("id = ?", id).Updates(map[string]any{"status": "pending", "last_error": ""})
		if result.Error != nil || result.RowsAffected == 0 {
			c.JSON(404, gin.H{"error": "Invalidation event not found"})
			return
		}
		c.JSON(200, gin.H{"message": "Invalidation queued"})
	}
}

func previewAdminCMSRestore() gin.HandlerFunc {
	return func(c *gin.Context) {
		var raw struct {
			SchemaVersion int   `json:"schema_version"`
			Pages         []any `json:"pages"`
			Navigation    []any `json:"navigation"`
			GlobalRegions []any `json:"global_regions"`
			Variants      []any `json:"variants"`
		}
		if err := bindStrictJSON(c, &raw); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		errors := []string{}
		if raw.SchemaVersion < 1 {
			errors = append(errors, "Unsupported schema version")
		}
		c.JSON(200, gin.H{"valid": len(errors) == 0, "schema_version": raw.SchemaVersion, "pages": len(raw.Pages), "navigation": len(raw.Navigation), "global_regions": len(raw.GlobalRegions), "variants": len(raw.Variants), "warnings": []string{}, "errors": errors})
	}
}
