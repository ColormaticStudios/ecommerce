package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	cmsservice "ecommerce/internal/services/cms"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func listAdminCMSPages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, limit, offset := parsePagination(c, 25)
		records, total, err := cmsservice.NewPageService(db).List(limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list CMS pages"})
			return
		}
		data := make([]apicontract.CmsPageResponse, 0, len(records))
		for _, record := range records {
			data = append(data, cmsPageResponse(&record))
		}
		page, _, _ := parsePagination(c, 25)
		c.JSON(http.StatusOK, apicontract.CmsPageListResponse{
			Data: data,
			Pagination: apicontract.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      int(total),
				TotalPages: int((total + int64(limit) - 1) / int64(limit)),
			},
		})
	}
}

func getAdminCMSLocales(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		locales, err := cmsservice.NewPageService(db).Locales()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CMS locales"})
			return
		}
		c.JSON(http.StatusOK, apicontract.CmsLocaleSettings{Locales: cmsLocales(locales)})
	}
}

func updateAdminCMSLocales(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsLocaleSettingsInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		inputs := make([]cmsservice.LocaleInput, 0, len(req.Locales))
		for _, locale := range req.Locales {
			inputs = append(inputs, cmsservice.LocaleInput{
				Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault,
				FallbackLocale: optionalStringValue(locale.FallbackLocale),
			})
		}
		locales, err := cmsservice.NewPageService(db).UpdateLocales(inputs, c.GetString("userID"))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, apicontract.CmsLocaleSettings{Locales: cmsLocales(locales)})
	}
}

func listAdminCMSPageVariants(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		variants, err := cmsservice.NewPageService(db).ListVariants(pageID)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsPageVariants(variants, false))
	}
}

func createAdminCMSPageVariant(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return saveAdminCMSPageVariant(db, mediaService, false)
}

func updateAdminCMSPageVariant(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return saveAdminCMSPageVariant(db, mediaService, true)
}

func saveAdminCMSPageVariant(db *gorm.DB, mediaService *media.Service, updating bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsPageVariantInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsservice.VariantInput{
			Locale: req.Locale, Market: optionalStringValue(req.Market), Path: req.Path,
			Slug: optionalStringValue(req.Slug), Title: req.Title, Payload: cmsPayloadToService(req.Payload),
			ChangeSummary: optionalStringValue(req.ChangeSummary), Actor: c.GetString("userID"),
		}
		var variant *models.CMSPageVariant
		if updating {
			variantID, parseErr := parsePositivePathID(c, "variant_id")
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid variant id"})
				return
			}
			variant, err = cmsservice.NewPageService(db, mediaService).UpdateVariant(pageID, variantID, input)
		} else {
			variant, err = cmsservice.NewPageService(db, mediaService).CreateVariant(pageID, input)
		}
		if err != nil {
			writeCMSError(c, err)
			return
		}
		status := http.StatusCreated
		if updating {
			status = http.StatusOK
		}
		c.JSON(status, cmsPageVariant(*variant, false))
	}
}

func deleteAdminCMSPageVariant(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, pageErr := parsePositivePathID(c, "id")
		variantID, variantErr := parsePositivePathID(c, "variant_id")
		if pageErr != nil || variantErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page or variant id"})
			return
		}
		if err := cmsservice.NewPageService(db, mediaService).DeleteVariant(pageID, variantID, c.GetString("userID")); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Page variant deleted"})
	}
}

func transitionAdminCMSPageVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID, pageErr := parsePositivePathID(c, "id")
		variantID, variantErr := parsePositivePathID(c, "variant_id")
		if pageErr != nil || variantErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page or variant id"})
			return
		}
		var req apicontract.CmsWorkflowActionInput
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			if err := bindStrictJSON(c, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		service := cmsservice.NewPageService(db)
		role, err := service.RoleForSubject(c.GetString("userID"))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		variant, err := service.TransitionVariantAsRole(
			pageID, variantID, c.Param("action"), c.GetString("userID"), role, optionalStringValue(req.Comment),
		)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsPageVariant(*variant, false))
	}
}

func listAdminCMSAuditEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryID, _ := strconv.ParseUint(c.Query("entry_id"), 10, 64)
		limit, _ := strconv.Atoi(c.Query("limit"))
		events, err := cmsservice.NewPageService(db).AuditEvents(uint(entryID), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CMS audit events"})
			return
		}
		result := make([]apicontract.CmsAuditEvent, 0, len(events))
		for _, event := range events {
			result = append(result, cmsAuditEvent(event))
		}
		c.JSON(http.StatusOK, result)
	}
}

func exportAdminCMSContent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageService := cmsservice.NewPageService(db)
		pages, _, err := pageService.List(10000, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CMS pages"})
			return
		}
		navigation, _, err := cmsservice.NewNavigationService(db).List(10000, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CMS navigation"})
			return
		}
		regions, _, err := cmsservice.NewGlobalRegionService(db).List(10000, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CMS global regions"})
			return
		}
		locales, err := pageService.Locales()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CMS locales"})
			return
		}
		pageResponses := make([]apicontract.CmsPageResponse, 0, len(pages))
		allVariants := make([]apicontract.CmsPageVariant, 0)
		for index := range pages {
			pageResponses = append(pageResponses, cmsPageResponse(&pages[index]))
			variants, variantErr := pageService.ListVariants(pages[index].Page.ID)
			if variantErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CMS variants"})
				return
			}
			allVariants = append(allVariants, cmsPageVariants(variants, false)...)
		}
		navigationResponses := make([]apicontract.CmsNavigationResponse, 0, len(navigation))
		for index := range navigation {
			navigationResponses = append(navigationResponses, cmsNavigationResponse(&navigation[index]))
		}
		regionResponses := make([]apicontract.CmsGlobalRegionResponse, 0, len(regions))
		for index := range regions {
			regionResponses = append(regionResponses, cmsGlobalRegionResponse(&regions[index]))
		}
		c.Header("Content-Disposition", "attachment; filename=cms-export.json")
		c.JSON(http.StatusOK, apicontract.CmsContentExport{
			SchemaVersion: 1, ExportedAt: time.Now().UTC(), Locales: cmsLocales(locales), Pages: pageResponses,
			Navigation: navigationResponses, GlobalRegions: regionResponses, Variants: allVariants,
		})
	}
}

func restoreAdminCMSContent(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		var req apicontract.CmsContentExport
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		raw, err := json.Marshal(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CMS export"})
			return
		}
		if err := cmsservice.NewPageService(db, mediaService).RestoreExport(raw, c.GetString("userID")); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "CMS content restored"})
	}
}

func createAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsPageDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsDraftInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewPageService(db, mediaService).CreateDraft(input)
		writeCMSMutationResponse(c, record, err, http.StatusCreated)
	}
}

func getAdminCMSPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		record, err := cmsservice.NewPageService(db).Get(id)
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func updateAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsPageDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsDraftInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewPageService(db, mediaService).UpdateDraft(id, input)
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func deleteAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		if err := cmsservice.NewPageService(db, mediaService).Delete(id, cmsActorID(db, c)); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "CMS page deleted"})
	}
}

func publishAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, roleErr := cmsservice.NewPageService(db).RoleForSubject(c.GetString("userID"))
		if roleErr != nil || role != "publisher" {
			c.JSON(http.StatusForbidden, gin.H{"error": cmsservice.ErrPermissionDenied.Error()})
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsPublishRequest
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			if err := bindStrictJSON(c, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		notes := ""
		if req.Notes != nil {
			notes = *req.Notes
		}
		record, err := cmsservice.NewPageService(db, mediaService).Publish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func unpublishAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		notes, ok := bindOptionalCMSPublishNotes(c)
		if !ok {
			return
		}
		record, err := cmsservice.NewPageService(db, mediaService).Unpublish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func discardAdminCMSPageDraft(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		record, deleted, err := cmsservice.NewPageService(db, mediaService).DiscardDraft(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: "Discarded from admin CMS editor"})
		if err != nil {
			writeCMSError(c, err)
			return
		}
		if deleted {
			c.Status(http.StatusNoContent)
			return
		}
		writeCMSMutationResponse(c, record, nil, http.StatusOK)
	}
}

func rollbackAdminCMSPage(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, roleErr := cmsservice.NewPageService(db).RoleForSubject(c.GetString("userID"))
		if roleErr != nil || role != "publisher" {
			c.JSON(http.StatusForbidden, gin.H{"error": cmsservice.ErrPermissionDenied.Error()})
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsRollbackRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		notes := ""
		if req.Notes != nil {
			notes = *req.Notes
		}
		record, err := cmsservice.NewPageService(db, mediaService).Rollback(id, cmsservice.RollbackInput{
			VersionID: uint(req.VersionId),
			ActorID:   cmsActorID(db, c),
			Notes:     notes,
		})
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func cmsActorID(db *gorm.DB, c *gin.Context) *uint {
	subject := strings.TrimSpace(c.GetString("userID"))
	if subject == "" {
		return nil
	}
	var user models.User
	if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
		return nil
	}
	return &user.ID
}

func requireCMSPublisher(c *gin.Context, db *gorm.DB) bool {
	role, err := cmsservice.NewPageService(db).RoleForSubject(c.GetString("userID"))
	if err != nil || role != "publisher" {
		c.JSON(http.StatusForbidden, gin.H{"error": cmsservice.ErrPermissionDenied.Error()})
		return false
	}
	return true
}

func getAdminCMSPageDelivery(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		record, err := cmsservice.NewPageService(db).GetDelivery(id)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsDeliveryResponse(record))
	}
}

func updateAdminCMSPageDelivery(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsPageDeliveryRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		record, err := cmsservice.NewPageService(db).UpdateDelivery(id, cmsDeliveryInput(req))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsDeliveryResponse(record))
	}
}

func getAdminCMSPageSEO(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		record, err := cmsservice.NewPageService(db).GetSEO(id)
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsSEOResponse(record))
	}
}

func updateAdminCMSPageSEO(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page id"})
			return
		}
		var req apicontract.CmsSEOInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		record, err := cmsservice.NewPageService(db, mediaService).UpdateSEO(id, cmsSEOInput(req))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsSEOResponse(record))
	}
}

func listAdminCMSRedirects(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := cmsservice.NewRedirectService(db).List()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list redirects"})
			return
		}
		result := make([]apicontract.CmsRedirectRule, 0, len(rules))
		for _, rule := range rules {
			result = append(result, cmsRedirectRule(rule))
		}
		c.JSON(http.StatusOK, result)
	}
}

func createAdminCMSRedirect(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsRedirectInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rule, err := cmsservice.NewRedirectService(db).Create(cmsRedirectInput(req))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusCreated, cmsRedirectRule(*rule))
	}
}

func updateAdminCMSRedirect(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redirect id"})
			return
		}
		var req apicontract.CmsRedirectInput
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rule, err := cmsservice.NewRedirectService(db).Update(id, cmsRedirectInput(req))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, cmsRedirectRule(*rule))
	}
}

func deleteAdminCMSRedirect(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redirect id"})
			return
		}
		if err := cmsservice.NewRedirectService(db).Delete(id); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Redirect deleted"})
	}
}

func resolveContentRedirect(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rule, target, err := cmsservice.NewRedirectService(db).Resolve(c.Query("path"))
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, apicontract.CmsRedirectResolution{TargetUrl: target, RedirectType: apicontract.CmsRedirectResolutionRedirectType(rule.RedirectType)})
	}
}

func getContentSitemap(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := c.GetHeader("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
		}
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		body, err := cmsservice.GenerateSitemap(db, scheme+"://"+host)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate sitemap"})
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
	}
}

func previewAdminCMSPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsPreviewRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		payload, err := cmsservice.ValidateAndNormalizePayload(cmsPayloadToService(req.Payload))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, apicontract.CmsPreviewResponse{
			Blocks: cmsPreviewBlocks(db, payload),
		})
	}
}

func resolveContentPage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		preview := isDraftPreviewActive(c)
		service := cmsservice.NewPageService(db)
		record, _, err := service.ResolveForLocale(c.Param("path"), c.Query("locale"), c.Query("market"), preview)
		if err == nil && !preview {
			requestContext := cmsRequestContext(c)
			decision, eligible, deliveryErr := service.ResolveDelivery(record, requestContext, time.Now().UTC())
			if deliveryErr != nil {
				err = deliveryErr
			} else if !eligible {
				err = cmsservice.ErrNotFound
			} else {
				version, versionErr := service.LoadVersion(record.Entry.ID, decision.ContentVersionID)
				if versionErr != nil {
					err = versionErr
				} else {
					record.PublishedVersion = version
					record.Delivery = decision
					c.Header("X-Correlation-ID", decision.CorrelationID)
					_ = service.RecordContentEvent(cmsservice.ContentEventInput{
						ContentVersionID: decision.ContentVersionID, ExperimentID: decision.ExperimentID,
						ExperimentVariantID: decision.ExperimentVariantID, CorrelationID: decision.CorrelationID,
						EventType: "impression",
					})
				}
			}
		}
		writeCMSMutationResponse(c, record, err, http.StatusOK)
	}
}

func recordContentEvent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsContentEventRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err := cmsservice.NewPageService(db).RecordContentEvent(cmsservice.ContentEventInput{
			ContentVersionID:    uint(req.ContentVersionId),
			ExperimentID:        uintPtrFromInt(req.ExperimentId),
			ExperimentVariantID: uintPtrFromInt(req.ExperimentVariantId),
			CorrelationID:       req.CorrelationId,
			EventType:           string(req.EventType),
		})
		if err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, apicontract.MessageResponse{Message: "Content event recorded"})
	}
}

func cmsRequestContext(c *gin.Context) cmsservice.RequestContext {
	correlationID := strings.TrimSpace(c.GetHeader("X-Correlation-ID"))
	if correlationID == "" {
		correlationID = uuid.NewString()
	}
	device := strings.TrimSpace(c.Query("device"))
	if device == "" {
		userAgent := strings.ToLower(c.GetHeader("User-Agent"))
		switch {
		case strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "tablet"):
			device = "tablet"
		case strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android"):
			device = "mobile"
		default:
			device = "desktop"
		}
	}
	authenticated, _ := c.Get("cms_authenticated")
	customerAssignment, _ := c.Get("cms_customer_assignment")
	customerKey, _ := customerAssignment.(string)
	return cmsservice.RequestContext{
		Market: c.Query("market"), DeviceClass: device, Authenticated: authenticated == true,
		Referrer: c.GetHeader("Referer"), UTMSource: c.Query("utm_source"), SegmentKey: c.Query("segment"),
		AssignmentKey: c.Query("assignment_key"), CustomerKey: customerKey, CorrelationID: correlationID,
	}
}

func cmsPreviewBlocks(db *gorm.DB, payload cmsservice.PagePayload) []apicontract.CmsPreviewBlock {
	rawBlocks, _ := payload["blocks"].([]any)
	blocks := make([]apicontract.CmsPreviewBlock, 0, len(rawBlocks))
	for index, raw := range rawBlocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		preview := apicontract.CmsPreviewBlock{
			Key:       fmt.Sprintf("%s:%d", blockType, index),
			Type:      blockType,
			Status:    apicontract.Static,
			ItemCount: 0,
			Messages:  []string{},
		}
		switch blockType {
		case "product_rail":
			preview.Status = apicontract.Ok
			preview.ItemCount = cmsPreviewProductCount(db, block)
			if preview.ItemCount == 0 {
				preview.Status = apicontract.Degraded
				preview.Messages = append(preview.Messages, "No published products matched this rail.")
			}
		case "category_tiles":
			preview.Status = apicontract.Ok
			preview.ItemCount = cmsPreviewCategoryCount(db, block)
			if preview.ItemCount == 0 {
				preview.Status = apicontract.Degraded
				preview.Messages = append(preview.Messages, "No active categories matched these slugs.")
			}
		case "inventory_message":
			preview.Status = apicontract.Ok
			if cmsPreviewProductExists(db, uint(numericFromMap(block, "product_id"))) {
				preview.ItemCount = 1
			} else {
				preview.Status = apicontract.Degraded
				preview.Messages = append(preview.Messages, "The referenced published product was not found.")
			}
		case "promotion_highlight":
			preview.Status = apicontract.Ok
			if !cmsPreviewPromotionExists(db, block) {
				preview.Status = apicontract.Degraded
				preview.Messages = append(preview.Messages, "No active promotion matched this reference.")
			} else {
				preview.ItemCount = 1
			}
		}
		blocks = append(blocks, preview)
	}
	return blocks
}

func cmsPreviewProductCount(db *gorm.DB, block map[string]any) int {
	var count int64
	query := db.Model(&models.Product{}).Where("is_published = ? AND deleted_at IS NULL", true)
	if source, _ := block["source"].(string); source == "manual" {
		ids := numericSliceFromMap(block, "product_ids")
		if len(ids) == 0 {
			return 0
		}
		query = query.Where("id IN ?", ids)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0
	}
	limit := numericFromMap(block, "limit")
	if limit > 0 && int(count) > limit {
		return limit
	}
	return int(count)
}

func cmsPreviewCategoryCount(db *gorm.DB, block map[string]any) int {
	slugs, _ := block["category_slugs"].([]string)
	if len(slugs) == 0 {
		return 0
	}
	var count int64
	if err := db.Model(&models.Category{}).Where("is_active = ? AND slug IN ?", true, slugs).Count(&count).Error; err != nil {
		return 0
	}
	return int(count)
}

func cmsPreviewProductExists(db *gorm.DB, id uint) bool {
	if id == 0 {
		return false
	}
	var count int64
	_ = db.Model(&models.Product{}).Where("id = ? AND is_published = ? AND deleted_at IS NULL", id, true).Count(&count).Error
	return count > 0
}

func cmsPreviewPromotionExists(db *gorm.DB, block map[string]any) bool {
	query := db.Model(&models.DiscountCampaign{}).Where("type = ? AND status IN ? AND is_archived = ?", models.DiscountCampaignTypePromotion, []string{models.DiscountCampaignStatusActive, models.DiscountCampaignStatusScheduled}, false)
	if id := numericFromMap(block, "campaign_id"); id > 0 {
		query = query.Where("id = ?", id)
	} else if code, ok := block["promotion_code"].(string); ok && code != "" {
		query = query.Where("coupon_code = ?", code)
	} else {
		return true
	}
	var count int64
	_ = query.Count(&count).Error
	return count > 0
}

func numericFromMap(record map[string]any, key string) int {
	switch value := record[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	default:
		return 0
	}
}

func numericSliceFromMap(record map[string]any, key string) []int {
	values, ok := record[key].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case int:
			out = append(out, typed)
		case int64:
			out = append(out, int(typed))
		case float64:
			out = append(out, int(typed))
		case float32:
			out = append(out, int(typed))
		}
	}
	return out
}

func listAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, offset := parsePagination(c, 25)
		records, total, err := cmsservice.NewNavigationService(db).List(limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list CMS navigation"})
			return
		}
		data := make([]apicontract.CmsNavigationResponse, 0, len(records))
		for _, record := range records {
			data = append(data, cmsNavigationResponse(&record))
		}
		c.JSON(http.StatusOK, apicontract.CmsNavigationListResponse{
			Data:       data,
			Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: int((total + int64(limit) - 1) / int64(limit))},
		})
	}
}

func createAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsNavigationDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsNavigationInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewNavigationService(db).CreateDraft(input)
		writeCMSNavigationResponse(c, record, err, http.StatusCreated)
	}
}

func getAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		record, err := cmsservice.NewNavigationService(db).Get(id)
		writeCMSNavigationResponse(c, record, err, http.StatusOK)
	}
}

func updateAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		var req apicontract.CmsNavigationDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsNavigationInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewNavigationService(db).UpdateDraft(id, input)
		writeCMSNavigationResponse(c, record, err, http.StatusOK)
	}
}

func deleteAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		if err := cmsservice.NewNavigationService(db).Delete(id, cmsActorID(db, c)); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "CMS navigation menu deleted"})
	}
}

func publishAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		notes, ok := bindOptionalCMSPublishNotes(c)
		if !ok {
			return
		}
		record, err := cmsservice.NewNavigationService(db).Publish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSNavigationResponse(c, record, err, http.StatusOK)
	}
}

func unpublishAdminCMSNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		notes, ok := bindOptionalCMSPublishNotes(c)
		if !ok {
			return
		}
		record, err := cmsservice.NewNavigationService(db).Unpublish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSNavigationResponse(c, record, err, http.StatusOK)
	}
}

func discardAdminCMSNavigationDraft(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid navigation id"})
			return
		}
		record, deleted, err := cmsservice.NewNavigationService(db).DiscardDraft(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: "Discarded from admin CMS editor"})
		if err != nil {
			writeCMSError(c, err)
			return
		}
		if deleted {
			c.Status(http.StatusNoContent)
			return
		}
		writeCMSNavigationResponse(c, record, nil, http.StatusOK)
	}
}

func resolveContentNavigation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		record, err := cmsservice.NewNavigationService(db).Resolve(c.Param("location"), isDraftPreviewActive(c))
		writeCMSNavigationResponse(c, record, err, http.StatusOK)
	}
}

func listAdminCMSGlobalRegions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, offset := parsePagination(c, 25)
		records, total, err := cmsservice.NewGlobalRegionService(db).List(limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list CMS global regions"})
			return
		}
		data := make([]apicontract.CmsGlobalRegionResponse, 0, len(records))
		for _, record := range records {
			data = append(data, cmsGlobalRegionResponse(&record))
		}
		c.JSON(http.StatusOK, apicontract.CmsGlobalRegionListResponse{
			Data:       data,
			Pagination: apicontract.Pagination{Page: page, Limit: limit, Total: int(total), TotalPages: int((total + int64(limit) - 1) / int64(limit))},
		})
	}
}

func createAdminCMSGlobalRegion(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apicontract.CmsGlobalRegionDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsGlobalRegionInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewGlobalRegionService(db, mediaService).CreateDraft(input)
		writeCMSGlobalRegionResponse(c, record, err, http.StatusCreated)
	}
}

func getAdminCMSGlobalRegion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		record, err := cmsservice.NewGlobalRegionService(db).Get(id)
		writeCMSGlobalRegionResponse(c, record, err, http.StatusOK)
	}
}

func updateAdminCMSGlobalRegion(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		var req apicontract.CmsGlobalRegionDraftRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input := cmsGlobalRegionInput(req)
		input.ActorID = cmsActorID(db, c)
		record, err := cmsservice.NewGlobalRegionService(db, mediaService).UpdateDraft(id, input)
		writeCMSGlobalRegionResponse(c, record, err, http.StatusOK)
	}
}

func deleteAdminCMSGlobalRegion(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		if err := cmsservice.NewGlobalRegionService(db, mediaService).Delete(id, cmsActorID(db, c)); err != nil {
			writeCMSError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "CMS global region deleted"})
	}
}

func publishAdminCMSGlobalRegion(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		notes, ok := bindOptionalCMSPublishNotes(c)
		if !ok {
			return
		}
		record, err := cmsservice.NewGlobalRegionService(db, mediaService).Publish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSGlobalRegionResponse(c, record, err, http.StatusOK)
	}
}

func unpublishAdminCMSGlobalRegion(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCMSPublisher(c, db) {
			return
		}
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		notes, ok := bindOptionalCMSPublishNotes(c)
		if !ok {
			return
		}
		record, err := cmsservice.NewGlobalRegionService(db, mediaService).Unpublish(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: notes})
		writeCMSGlobalRegionResponse(c, record, err, http.StatusOK)
	}
}

func discardAdminCMSGlobalRegionDraft(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid global region id"})
			return
		}
		record, deleted, err := cmsservice.NewGlobalRegionService(db, mediaService).DiscardDraft(id, cmsservice.PublishInput{ActorID: cmsActorID(db, c), Notes: "Discarded from admin CMS editor"})
		if err != nil {
			writeCMSError(c, err)
			return
		}
		if deleted {
			c.Status(http.StatusNoContent)
			return
		}
		writeCMSGlobalRegionResponse(c, record, nil, http.StatusOK)
	}
}

func resolveContentGlobalRegion(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		record, err := cmsservice.NewGlobalRegionService(db).Resolve(c.Param("region"), isDraftPreviewActive(c))
		writeCMSGlobalRegionResponse(c, record, err, http.StatusOK)
	}
}

func writeCMSMutationResponse(c *gin.Context, record *cmsservice.PageRecord, err error, successStatus int) {
	if err == nil {
		c.JSON(successStatus, cmsPageResponse(record))
		return
	}
	switch {
	case errors.Is(err, cmsservice.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS page not found"})
	case errors.Is(err, cmsservice.ErrDuplicatePath):
		c.JSON(http.StatusConflict, gin.H{"error": "CMS page path already exists"})
	case errors.Is(err, cmsservice.ErrInvalidPage), errors.Is(err, cmsservice.ErrNoDraft), errors.Is(err, cmsservice.ErrInvalidDelivery), errors.Is(err, cmsservice.ErrRedirectLoop):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CMS page operation failed"})
	}
}

func writeCMSNavigationResponse(c *gin.Context, record *cmsservice.NavigationRecord, err error, successStatus int) {
	if err == nil {
		c.JSON(successStatus, cmsNavigationResponse(record))
		return
	}
	writeCMSError(c, err)
}

func writeCMSGlobalRegionResponse(c *gin.Context, record *cmsservice.GlobalRegionRecord, err error, successStatus int) {
	if err == nil {
		c.JSON(successStatus, cmsGlobalRegionResponse(record))
		return
	}
	writeCMSError(c, err)
}

func writeCMSError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, cmsservice.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "CMS content not found"})
	case errors.Is(err, cmsservice.ErrDuplicatePath):
		c.JSON(http.StatusConflict, gin.H{"error": "CMS content key already exists"})
	case errors.Is(err, cmsservice.ErrDuplicateVariant):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, cmsservice.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, cmsservice.ErrInvalidExport):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, cmsservice.ErrInvalidPage), errors.Is(err, cmsservice.ErrNoDraft), errors.Is(err, cmsservice.ErrInvalidDelivery), errors.Is(err, cmsservice.ErrRedirectLoop), errors.Is(err, cmsservice.ErrInvalidLocale), errors.Is(err, cmsservice.ErrInvalidTransition), errors.Is(err, cmsservice.ErrApprovalRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "CMS content operation failed"})
	}
}

func bindOptionalCMSPublishNotes(c *gin.Context) (string, bool) {
	var req apicontract.CmsPublishRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return "", false
		}
	}
	if req.Notes != nil {
		return *req.Notes, true
	}
	return "", true
}

func cmsDraftInput(req apicontract.CmsPageDraftRequest) cmsservice.PageDraftInput {
	input := cmsservice.PageDraftInput{
		Path:    req.Path,
		Title:   req.Title,
		Payload: cmsPayloadToService(req.Payload),
	}
	if req.Slug != nil {
		input.Slug = *req.Slug
	}
	if req.TemplateKey != nil {
		input.TemplateKey = *req.TemplateKey
	}
	if req.Visibility != nil {
		input.Visibility = string(*req.Visibility)
	}
	if req.IsHomepage != nil {
		input.IsHomepage = *req.IsHomepage
	}
	if req.ChangeSummary != nil {
		input.ChangeSummary = *req.ChangeSummary
	}
	return input
}

func cmsNavigationInput(req apicontract.CmsNavigationDraftRequest) cmsservice.NavigationDraftInput {
	items := make([]cmsservice.NavigationItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, cmsservice.NavigationItemInput{
			ID:        uintValueFromInt(item.Id),
			ParentID:  uintPtrFromInt(item.ParentId),
			Label:     item.Label,
			ItemType:  string(item.ItemType),
			TargetRef: item.TargetRef,
			URL:       item.Url,
			SortOrder: item.SortOrder,
			IsEnabled: item.IsEnabled,
		})
	}
	input := cmsservice.NavigationDraftInput{
		Key:      req.Key,
		Title:    req.Title,
		Location: req.Location,
		Items:    items,
	}
	if req.ChangeSummary != nil {
		input.ChangeSummary = *req.ChangeSummary
	}
	return input
}

func cmsGlobalRegionInput(req apicontract.CmsGlobalRegionDraftRequest) cmsservice.GlobalRegionDraftInput {
	input := cmsservice.GlobalRegionDraftInput{
		Key:     req.Key,
		Title:   req.Title,
		Region:  req.Region,
		Payload: cmsPayloadToService(req.Payload),
	}
	if req.ChangeSummary != nil {
		input.ChangeSummary = *req.ChangeSummary
	}
	return input
}

func cmsPageResponse(record *cmsservice.PageRecord) apicontract.CmsPageResponse {
	response := apicontract.CmsPageResponse{
		Page:                cmsPage(record.Page),
		Entry:               cmsEntry(record.Entry),
		CurrentVersion:      cmsVersion(record.CurrentVersion),
		PublishedVersion:    cmsVersion(record.PublishedVersion),
		LatestPublication:   cmsPublication(record.LatestPublication),
		HasUnpublishedDraft: record.HasUnpublishedDraft,
	}
	if record.Delivery != nil {
		response.Delivery = &apicontract.CmsDeliveryDecision{
			ContentVersionId:    int(record.Delivery.ContentVersionID),
			ExperimentId:        cmsOptionalInt(record.Delivery.ExperimentID),
			ExperimentVariantId: cmsOptionalInt(record.Delivery.ExperimentVariantID),
			CorrelationId:       record.Delivery.CorrelationID,
		}
	}
	if record.SEO != nil {
		seo := cmsSEOMetadata(*record.SEO)
		response.Seo = &seo
	}
	if record.Localization != nil {
		alternates := make([]struct {
			Locale string  `json:"locale"`
			Market *string `json:"market,omitempty"`
			Path   string  `json:"path"`
		}, 0, len(record.Localization.Alternates))
		for _, variant := range record.Localization.Alternates {
			market := variant.Market
			alternates = append(alternates, struct {
				Locale string  `json:"locale"`
				Market *string `json:"market,omitempty"`
				Path   string  `json:"path"`
			}{Locale: variant.Locale, Market: &market, Path: variant.Path})
		}
		response.Localization = &apicontract.CmsResolvedLocalization{
			RequestedLocale: record.Localization.RequestedLocale, ResolvedLocale: record.Localization.ResolvedLocale,
			Market: record.Localization.Market, UsedFallback: record.Localization.UsedFallback, Alternates: alternates,
		}
	}
	return response
}

func cmsLocales(locales []models.CMSLocale) []apicontract.CmsLocale {
	result := make([]apicontract.CmsLocale, 0, len(locales))
	for _, locale := range locales {
		fallback := locale.FallbackLocale
		result = append(result, apicontract.CmsLocale{
			Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault, FallbackLocale: &fallback,
		})
	}
	return result
}

func cmsPageVariants(variants []models.CMSPageVariant, published bool) []apicontract.CmsPageVariant {
	result := make([]apicontract.CmsPageVariant, 0, len(variants))
	for _, variant := range variants {
		result = append(result, cmsPageVariant(variant, published))
	}
	return result
}

func cmsPageVariant(variant models.CMSPageVariant, published bool) apicontract.CmsPageVariant {
	payload := variant.DraftPayloadJSON
	if published {
		payload = variant.PublishedPayloadJSON
	}
	submittedBy, approvedBy := variant.SubmittedBy, variant.ApprovedBy
	return apicontract.CmsPageVariant{
		Id: int(variant.ID), PageId: int(variant.PageID), EntryId: int(variant.EntryID), Locale: variant.Locale,
		Market: variant.Market, Path: variant.Path, Slug: variant.Slug, Title: variant.Title,
		Payload: cmsPayloadFromJSON(payload), Status: apicontract.CmsPageVariantStatus(variant.Status), Revision: int(variant.Revision),
		SubmittedBy: &submittedBy, ApprovedBy: &approvedBy, PublishedAt: variant.PublishedAt,
		CreatedAt: variant.CreatedAt, UpdatedAt: variant.UpdatedAt,
	}
}

func cmsAuditEvent(event models.CMSAuditEvent) apicontract.CmsAuditEvent {
	return apicontract.CmsAuditEvent{
		Id: int(event.ID), EntryId: int(event.EntryID), VersionId: cmsOptionalInt(event.VersionID),
		VariantId: cmsOptionalInt(event.VariantID), Action: event.Action, Actor: event.Actor,
		Detail: event.Detail, CreatedAt: event.CreatedAt,
	}
}

func cmsSEOInput(req apicontract.CmsSEOInput) cmsservice.SEOInput {
	return cmsservice.SEOInput{
		Title: req.Title, Description: req.Description, CanonicalURL: req.CanonicalUrl, Robots: string(req.Robots),
		OGTitle: req.OgTitle, OGDescription: req.OgDescription, OGImageMediaID: req.OgImageMediaId,
		TwitterCard: string(req.TwitterCard), TwitterTitle: req.TwitterTitle,
		TwitterDescription: req.TwitterDescription, TwitterImageMediaID: req.TwitterImageMediaId, JSONLD: req.JsonLd,
	}
}

func cmsSEOResponse(record *cmsservice.SEORecord) apicontract.CmsSEOResponse {
	return apicontract.CmsSEOResponse{Metadata: cmsSEOMetadata(record.Metadata), Issues: record.Issues}
}

func cmsSEOMetadata(metadata models.SEOMetadata) apicontract.CmsSEOMetadata {
	jsonLD := []map[string]any{}
	_ = json.Unmarshal([]byte(metadata.JSONLD), &jsonLD)
	return apicontract.CmsSEOMetadata{
		Title: optionalStringValue(metadata.Title), Description: optionalStringValue(metadata.Description), CanonicalUrl: optionalStringValue(metadata.CanonicalPath),
		Robots: apicontract.CmsSEOMetadataRobots(metadata.Robots), OgTitle: optionalStringValue(metadata.OGTitle),
		OgDescription: optionalStringValue(metadata.OGDescription), OgImageMediaId: metadata.OgImageMediaID,
		TwitterCard: apicontract.CmsSEOMetadataTwitterCard(metadata.TwitterCard), TwitterTitle: optionalStringValue(metadata.TwitterTitle),
		TwitterDescription: optionalStringValue(metadata.TwitterDescription), TwitterImageMediaId: metadata.TwitterImageMediaID, JsonLd: jsonLD,
	}
}

func cmsRedirectInput(req apicontract.CmsRedirectInput) cmsservice.RedirectInput {
	return cmsservice.RedirectInput{SourcePattern: req.SourcePattern, MatchType: string(req.MatchType), TargetURL: req.TargetUrl, RedirectType: int(req.RedirectType), Priority: req.Priority, IsEnabled: req.IsEnabled}
}

func cmsRedirectRule(rule models.CMSRedirectRule) apicontract.CmsRedirectRule {
	return apicontract.CmsRedirectRule{Id: int(rule.ID), SourcePattern: rule.SourcePattern, MatchType: apicontract.CmsRedirectRuleMatchType(rule.MatchType), TargetUrl: rule.TargetURL, RedirectType: apicontract.CmsRedirectRuleRedirectType(rule.RedirectType), Priority: rule.Priority, IsEnabled: rule.IsEnabled, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt}
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cmsDeliveryInput(req apicontract.CmsPageDeliveryRequest) cmsservice.DeliveryInput {
	input := cmsservice.DeliveryInput{TargetingRules: make([]cmsservice.TargetingRuleInput, 0, len(req.TargetingRules))}
	if req.Schedule != nil {
		input.Schedule = &cmsservice.ScheduleInput{
			PublishAt: req.Schedule.PublishAt, UnpublishAt: req.Schedule.UnpublishAt, Timezone: req.Schedule.Timezone,
		}
	}
	for _, rule := range req.TargetingRules {
		input.TargetingRules = append(input.TargetingRules, cmsservice.TargetingRuleInput{
			TargetingRule: cmsservice.TargetingRule{
				Markets: rule.Markets, DeviceClasses: stringValues(rule.DeviceClasses), AuthStates: stringValues(rule.AuthStates),
				Referrers: rule.Referrers, UTMSources: rule.UtmSources, SegmentKeys: rule.SegmentKeys,
			},
			Priority: rule.Priority, IsEnabled: rule.IsEnabled,
		})
	}
	if req.Experiment != nil {
		variants := make([]cmsservice.ExperimentVariantInput, 0, len(req.Experiment.Variants))
		for _, variant := range req.Experiment.Variants {
			variants = append(variants, cmsservice.ExperimentVariantInput{Name: variant.Name, VersionID: uint(variant.VersionId), Allocation: variant.Allocation})
		}
		input.Experiment = &cmsservice.ExperimentInput{
			Name: req.Experiment.Name, Status: models.CMSExperimentStatus(req.Experiment.Status), StickyKey: string(req.Experiment.StickyKey),
			StartsAt: req.Experiment.StartsAt, EndsAt: req.Experiment.EndsAt, Variants: variants,
		}
	}
	return input
}

func cmsDeliveryResponse(record *cmsservice.DeliveryRecord) apicontract.CmsPageDeliveryResponse {
	response := apicontract.CmsPageDeliveryResponse{
		TargetingRules: []apicontract.CmsTargetingRule{}, RecentPublications: []apicontract.CmsPublication{},
	}
	if record.Schedule != nil {
		response.Schedule = &apicontract.CmsSchedule{
			Id: int(record.Schedule.ID), EntryId: int(record.Schedule.EntryID), VersionId: int(record.Schedule.VersionID),
			PublishAt: record.Schedule.PublishAt, UnpublishAt: record.Schedule.UnpublishAt, Timezone: record.Schedule.Timezone,
			Status: apicontract.CmsScheduleStatus(record.Schedule.Status), LastTransitionAt: record.Schedule.LastTransitionAt,
			CreatedAt: record.Schedule.CreatedAt, UpdatedAt: record.Schedule.UpdatedAt,
		}
	}
	for _, rule := range record.TargetingRules {
		response.TargetingRules = append(response.TargetingRules, apicontract.CmsTargetingRule{
			Id: int(rule.Model.ID), Priority: rule.Model.Priority, IsEnabled: rule.Model.IsEnabled,
			Markets: rule.Rule.Markets, DeviceClasses: cmsDeviceValues(rule.Rule.DeviceClasses),
			AuthStates: cmsAuthValues(rule.Rule.AuthStates), Referrers: rule.Rule.Referrers,
			UtmSources: rule.Rule.UTMSources, SegmentKeys: rule.Rule.SegmentKeys,
		})
	}
	if record.Experiment != nil {
		variants := make([]apicontract.CmsExperimentVariant, 0, len(record.Experiment.Variants))
		for _, variant := range record.Experiment.Variants {
			variants = append(variants, apicontract.CmsExperimentVariant{
				Id: int(variant.ID), ExperimentId: int(variant.ExperimentID), Name: variant.Name,
				VersionId: int(variant.VersionID), Allocation: variant.Allocation,
			})
		}
		response.Experiment = &apicontract.CmsExperiment{
			Id: int(record.Experiment.ID), EntryId: int(record.Experiment.EntryID), Name: record.Experiment.Name,
			Status: apicontract.CmsExperimentStatus(record.Experiment.Status), StickyKey: apicontract.CmsExperimentStickyKey(record.Experiment.StickyKey),
			StartsAt: record.Experiment.StartsAt, EndsAt: record.Experiment.EndsAt, Variants: variants,
			CreatedAt: record.Experiment.CreatedAt, UpdatedAt: record.Experiment.UpdatedAt,
		}
	}
	for index := range record.RecentPublications {
		if publication := cmsPublication(&record.RecentPublications[index]); publication != nil {
			response.RecentPublications = append(response.RecentPublications, *publication)
		}
	}
	return response
}

func stringValues[T ~string](values []T) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func cmsDeviceValues(values []string) []apicontract.CmsTargetingRuleDeviceClasses {
	result := make([]apicontract.CmsTargetingRuleDeviceClasses, len(values))
	for index, value := range values {
		result[index] = apicontract.CmsTargetingRuleDeviceClasses(value)
	}
	return result
}

func cmsAuthValues(values []string) []apicontract.CmsTargetingRuleAuthStates {
	result := make([]apicontract.CmsTargetingRuleAuthStates, len(values))
	for index, value := range values {
		result[index] = apicontract.CmsTargetingRuleAuthStates(value)
	}
	return result
}

func cmsNavigationResponse(record *cmsservice.NavigationRecord) apicontract.CmsNavigationResponse {
	items := make([]apicontract.CmsNavigationItem, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, cmsNavigationItem(item))
	}
	return apicontract.CmsNavigationResponse{
		Menu:                cmsNavigationMenu(record.Menu),
		Entry:               cmsEntry(record.Entry),
		Items:               items,
		CurrentVersion:      cmsVersion(record.CurrentVersion),
		PublishedVersion:    cmsVersion(record.PublishedVersion),
		LatestPublication:   cmsPublication(record.LatestPublication),
		HasUnpublishedDraft: record.HasUnpublishedDraft,
	}
}

func cmsGlobalRegionResponse(record *cmsservice.GlobalRegionRecord) apicontract.CmsGlobalRegionResponse {
	return apicontract.CmsGlobalRegionResponse{
		Region:              cmsGlobalRegion(record.Region),
		Entry:               cmsEntry(record.Entry),
		CurrentVersion:      cmsVersion(record.CurrentVersion),
		PublishedVersion:    cmsVersion(record.PublishedVersion),
		LatestPublication:   cmsPublication(record.LatestPublication),
		HasUnpublishedDraft: record.HasUnpublishedDraft,
	}
}

func cmsEntry(entry models.CMSEntry) apicontract.CmsEntry {
	return apicontract.CmsEntry{
		Id:                 int(entry.ID),
		EntryType:          apicontract.CmsEntryEntryType(entry.EntryType),
		Key:                entry.Key,
		Status:             apicontract.CmsEntryStatus(entry.Status),
		CurrentVersionId:   cmsOptionalInt(entry.CurrentVersionID),
		PublishedVersionId: cmsOptionalInt(entry.PublishedVersionID),
		CreatedAt:          entry.CreatedAt,
		UpdatedAt:          entry.UpdatedAt,
	}
}

func cmsPage(page models.CMSPage) apicontract.CmsPage {
	return apicontract.CmsPage{
		Id:            int(page.ID),
		EntryId:       int(page.EntryID),
		Path:          page.Path,
		Slug:          page.Slug,
		Title:         page.Title,
		TemplateKey:   page.TemplateKey,
		Visibility:    apicontract.CmsPageVisibility(page.Visibility),
		SeoMetadataId: cmsOptionalInt(page.SEOMetadataID),
		IsHomepage:    page.IsHomepage,
		CreatedAt:     page.CreatedAt,
		UpdatedAt:     page.UpdatedAt,
	}
}

func cmsNavigationMenu(menu models.CMSNavigationMenu) apicontract.CmsNavigationMenu {
	return apicontract.CmsNavigationMenu{
		Id:        int(menu.ID),
		EntryId:   int(menu.EntryID),
		Key:       menu.Key,
		Title:     menu.Title,
		Location:  menu.Location,
		CreatedAt: menu.CreatedAt,
		UpdatedAt: menu.UpdatedAt,
	}
}

func cmsNavigationItem(item models.CMSNavigationItem) apicontract.CmsNavigationItem {
	return apicontract.CmsNavigationItem{
		Id:        int(item.ID),
		MenuId:    int(item.MenuID),
		ParentId:  cmsOptionalInt(item.ParentID),
		Label:     item.Label,
		ItemType:  apicontract.CmsNavigationItemItemType(item.ItemType),
		TargetRef: item.TargetRef,
		Url:       item.URL,
		SortOrder: item.SortOrder,
		IsEnabled: item.IsEnabled,
	}
}

func cmsGlobalRegion(region models.CMSGlobalRegion) apicontract.CmsGlobalRegion {
	return apicontract.CmsGlobalRegion{
		Id:        int(region.ID),
		EntryId:   int(region.EntryID),
		Key:       region.Key,
		Title:     region.Title,
		Region:    region.Region,
		CreatedAt: region.CreatedAt,
		UpdatedAt: region.UpdatedAt,
	}
}

func cmsVersion(version *models.CMSEntryVersion) *apicontract.CmsEntryVersion {
	if version == nil {
		return nil
	}
	return &apicontract.CmsEntryVersion{
		Id:            int(version.ID),
		EntryId:       int(version.EntryID),
		VersionNumber: int(version.VersionNumber),
		SchemaVersion: int(version.SchemaVersion),
		Payload:       cmsPayloadFromJSON(version.PayloadJSON),
		CreatedBy:     cmsOptionalInt(version.CreatedBy),
		ChangeSummary: &version.ChangeSummary,
		CreatedAt:     version.CreatedAt,
	}
}

func cmsPublication(publication *models.CMSPublication) *apicontract.CmsPublication {
	if publication == nil {
		return nil
	}
	return &apicontract.CmsPublication{
		Id:                        int(publication.ID),
		EntryId:                   int(publication.EntryID),
		VersionId:                 int(publication.VersionID),
		PublishedBy:               cmsOptionalInt(publication.PublishedBy),
		PublishedAt:               publication.PublishedAt,
		RollbackFromPublicationId: cmsOptionalInt(publication.RollbackFromPublicationID),
		Notes:                     &publication.Notes,
	}
}

func cmsPayloadToService(payload apicontract.CmsPagePayload) cmsservice.PagePayload {
	raw, err := json.Marshal(payload)
	if err != nil {
		return cmsservice.PagePayload{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return cmsservice.PagePayload{}
	}
	return cmsservice.PagePayload(out)
}

func cmsPayloadFromJSON(raw string) apicontract.CmsPagePayload {
	if raw == "" {
		return apicontract.CmsPagePayload{}
	}
	var out apicontract.CmsPagePayload
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return apicontract.CmsPagePayload{}
	}
	return out
}

func cmsOptionalInt(value *uint) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

func uintPtrFromInt(value *int) *uint {
	if value == nil {
		return nil
	}
	converted := uint(*value)
	return &converted
}

func uintValueFromInt(value *int) uint {
	if value == nil {
		return 0
	}
	return uint(*value)
}
