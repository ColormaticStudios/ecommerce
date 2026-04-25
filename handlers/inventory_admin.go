package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/apicontract"
	inventoryservice "ecommerce/internal/services/inventory"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListAdminInventoryReservations(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses := parseReservationStatuses(c.QueryArray("status"))
		limit := parseOptionalLimit(c.Query("limit"), 100, 200)

		reservations, err := inventoryservice.ListReservations(db, statuses, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load inventory reservations"})
			return
		}

		items := make([]apicontract.InventoryReservation, 0, len(reservations))
		for _, reservation := range reservations {
			items = append(items, inventoryReservationContract(reservation))
		}

		c.JSON(http.StatusOK, apicontract.InventoryReservationList{Items: items})
	}
}

func ListAdminInventoryAlerts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		statuses := parseInventoryFilterValues(c.QueryArray("status"))
		limit := parseOptionalLimit(c.Query("limit"), 100, 200)

		alerts, err := inventoryservice.ListAlerts(db, statuses, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load inventory alerts"})
			return
		}

		items := make([]apicontract.InventoryAlert, 0, len(alerts))
		for _, alert := range alerts {
			items = append(items, inventoryAlertContract(alert))
		}
		c.JSON(http.StatusOK, apicontract.InventoryAlertList{Items: items})
	}
}

func AckAdminInventoryAlert(db *gorm.DB) gin.HandlerFunc {
	return inventoryAlertAction(db, func(alertID uint, actor inventoryservice.AlertActionInput) (models.InventoryAlert, error) {
		return inventoryservice.AckAlert(db, alertID, actor)
	})
}

func ResolveAdminInventoryAlert(db *gorm.DB) gin.HandlerFunc {
	return inventoryAlertAction(db, func(alertID uint, actor inventoryservice.AlertActionInput) (models.InventoryAlert, error) {
		return inventoryservice.ResolveAlert(db, alertID, actor)
	})
}

func ListAdminInventoryThresholds(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var variantID *uint
		if raw := strings.TrimSpace(c.Query("product_variant_id")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product_variant_id"})
				return
			}
			id := uint(parsed)
			variantID = &id
		}
		thresholds, err := inventoryservice.GetThresholds(db, variantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load inventory thresholds"})
			return
		}
		items := make([]apicontract.InventoryThreshold, 0, len(thresholds))
		for _, threshold := range thresholds {
			items = append(items, apicontract.InventoryThreshold{
				Id:               int(threshold.ID),
				ProductVariantId: optionalInt(threshold.ProductVariantID),
				LowStockQuantity: threshold.LowStockQuantity,
				CreatedAt:        threshold.CreatedAt,
				UpdatedAt:        threshold.UpdatedAt,
			})
		}
		c.JSON(http.StatusOK, apicontract.InventoryThresholdList{Items: items})
	}
}

func UpsertAdminInventoryThreshold(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request apicontract.InventoryThresholdRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid threshold request"})
			return
		}
		var variantID *uint
		if request.ProductVariantId != nil {
			if *request.ProductVariantId < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product_variant_id"})
				return
			}
			id := uint(*request.ProductVariantId)
			variantID = &id
		}
		threshold, err := inventoryservice.SetThreshold(db, inventoryservice.ThresholdInput{
			ProductVariantID: variantID,
			LowStockQuantity: request.LowStockQuantity,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, apicontract.InventoryThreshold{
			Id:               int(threshold.ID),
			ProductVariantId: optionalInt(threshold.ProductVariantID),
			LowStockQuantity: threshold.LowStockQuantity,
			CreatedAt:        threshold.CreatedAt,
			UpdatedAt:        threshold.UpdatedAt,
		})
	}
}

func DeleteAdminInventoryThreshold(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid threshold id"})
			return
		}
		if err := inventoryservice.DeleteThreshold(db, uint(id)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete inventory threshold"})
			return
		}
		c.JSON(http.StatusOK, apicontract.MessageResponse{Message: "Inventory threshold deleted"})
	}
}

func CreateAdminInventoryAdjustment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request apicontract.InventoryAdjustmentRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid adjustment request"})
			return
		}
		if request.ProductVariantId < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product_variant_id"})
			return
		}
		input := inventoryservice.AdjustmentInput{
			ProductVariantID: uint(request.ProductVariantId),
			QuantityDelta:    request.QuantityDelta,
			ReasonCode:       string(request.ReasonCode),
			ActorType:        "admin",
		}
		if request.Notes != nil {
			input.Notes = *request.Notes
		}
		if request.ApprovedByType != nil {
			input.ApprovedByType = *request.ApprovedByType
		}
		if request.ApprovedById != nil {
			id := uint(*request.ApprovedById)
			input.ApprovedByID = &id
		}
		adjustment, availability, err := inventoryservice.CreateAdjustment(db, input, inventoryservice.AdjustmentPolicy{})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, apicontract.InventoryAdjustmentResponse{
			Adjustment:   inventoryAdjustmentContract(adjustment),
			Availability: inventoryAvailabilityContract(availability),
		})
	}
}

func RunAdminInventoryReconciliation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		report, err := inventoryservice.Reconcile(db, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run inventory reconciliation"})
			return
		}
		issues := make([]apicontract.InventoryReconciliationIssue, 0, len(report.Issues))
		for _, issue := range report.Issues {
			issues = append(issues, apicontract.InventoryReconciliationIssue{
				IssueType:        issue.IssueType,
				InventoryItemId:  int(issue.InventoryItemID),
				ProductVariantId: int(issue.ProductVariantID),
				Expected:         issue.Expected,
				Actual:           issue.Actual,
				Message:          issue.Message,
				EntityType:       issue.EntityType,
				EntityId:         optionalInt(issue.EntityID),
			})
		}
		c.JSON(http.StatusOK, apicontract.InventoryReconciliationReport{
			CheckedAt: report.CheckedAt,
			Issues:    issues,
		})
	}
}

func GetAdminInventoryTimeline(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID, err := strconv.Atoi(c.Param("product_variant_id"))
		if err != nil || variantID < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product variant id"})
			return
		}
		timeline, err := inventoryservice.GetTimeline(db, uint(variantID), parseOptionalLimit(c.Query("limit"), 50, 200))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		movements := make([]apicontract.InventoryMovement, 0, len(timeline.Movements))
		for _, movement := range timeline.Movements {
			movements = append(movements, inventoryMovementContract(movement))
		}
		reservations := make([]apicontract.InventoryReservation, 0, len(timeline.Reservations))
		for _, reservation := range timeline.Reservations {
			reservations = append(reservations, inventoryReservationContract(reservation))
		}
		adjustments := make([]apicontract.InventoryAdjustment, 0, len(timeline.Adjustments))
		for _, adjustment := range timeline.Adjustments {
			adjustments = append(adjustments, inventoryAdjustmentContract(adjustment))
		}
		c.JSON(http.StatusOK, apicontract.InventoryTimeline{
			ProductVariantId: int(timeline.ProductVariantID),
			Movements:        movements,
			Reservations:     reservations,
			Adjustments:      adjustments,
		})
	}
}

func parseReservationStatuses(values []string) []string {
	return parseInventoryFilterValues(values)
}

func parseInventoryFilterValues(values []string) []string {
	statuses := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			status := strings.ToUpper(strings.TrimSpace(part))
			if status != "" {
				statuses = append(statuses, status)
			}
		}
	}
	return statuses
}

func inventoryAlertAction(db *gorm.DB, action func(uint, inventoryservice.AlertActionInput) (models.InventoryAlert, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alert id"})
			return
		}
		alert, err := action(uint(id), inventoryservice.AlertActionInput{
			ActorType: "admin",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update inventory alert"})
			return
		}
		c.JSON(http.StatusOK, inventoryAlertContract(alert))
	}
}

func inventoryAlertContract(alert models.InventoryAlert) apicontract.InventoryAlert {
	return apicontract.InventoryAlert{
		Id:               int(alert.ID),
		ProductVariantId: int(alert.ProductVariantID),
		AlertType:        apicontract.InventoryAlertAlertType(alert.AlertType),
		Status:           apicontract.InventoryAlertStatus(alert.Status),
		Available:        alert.Available,
		Threshold:        alert.Threshold,
		OpenedAt:         alert.OpenedAt,
		AckedAt:          alert.AckedAt,
		AckedByType:      optionalString(alert.AckedByType),
		AckedById:        optionalInt(alert.AckedByID),
		ResolvedAt:       alert.ResolvedAt,
		ResolvedByType:   optionalString(alert.ResolvedByType),
		ResolvedById:     optionalInt(alert.ResolvedByID),
		CreatedAt:        alert.CreatedAt,
		UpdatedAt:        alert.UpdatedAt,
	}
}

func inventoryReservationContract(reservation models.InventoryReservation) apicontract.InventoryReservation {
	return apicontract.InventoryReservation{
		Id:                int(reservation.ID),
		ProductVariantId:  int(reservation.ProductVariantID),
		Quantity:          reservation.Quantity,
		Status:            apicontract.InventoryReservationStatus(reservation.Status),
		ExpiresAt:         reservation.ExpiresAt,
		OwnerType:         reservation.OwnerType,
		OwnerId:           optionalInt(reservation.OwnerID),
		CheckoutSessionId: optionalInt(reservation.CheckoutSessionID),
		OrderId:           optionalInt(reservation.OrderID),
		CreatedAt:         reservation.CreatedAt,
		UpdatedAt:         reservation.UpdatedAt,
	}
}

func inventoryMovementContract(movement models.InventoryMovement) apicontract.InventoryMovement {
	return apicontract.InventoryMovement{
		Id:              int(movement.ID),
		InventoryItemId: int(movement.InventoryItemID),
		MovementType:    movement.MovementType,
		QuantityDelta:   movement.QuantityDelta,
		ReferenceType:   movement.ReferenceType,
		ReferenceId:     optionalInt(movement.ReferenceID),
		ReasonCode:      movement.ReasonCode,
		ActorType:       movement.ActorType,
		ActorId:         optionalInt(movement.ActorID),
		CreatedAt:       movement.CreatedAt,
		UpdatedAt:       movement.UpdatedAt,
	}
}

func inventoryAdjustmentContract(adjustment models.InventoryAdjustment) apicontract.InventoryAdjustment {
	return apicontract.InventoryAdjustment{
		Id:               int(adjustment.ID),
		InventoryItemId:  int(adjustment.InventoryItemID),
		ProductVariantId: int(adjustment.ProductVariantID),
		QuantityDelta:    adjustment.QuantityDelta,
		ReasonCode:       apicontract.InventoryAdjustmentReason(adjustment.ReasonCode),
		Notes:            adjustment.Notes,
		ActorType:        adjustment.ActorType,
		ActorId:          optionalInt(adjustment.ActorID),
		ApprovedByType:   adjustment.ApprovedByType,
		ApprovedById:     optionalInt(adjustment.ApprovedByID),
		ApprovedAt:       adjustment.ApprovedAt,
		CreatedAt:        adjustment.CreatedAt,
		UpdatedAt:        adjustment.UpdatedAt,
	}
}

func inventoryAvailabilityContract(availability inventoryservice.Availability) apicontract.InventoryAvailability {
	return apicontract.InventoryAvailability{
		ProductVariantId: int(availability.ProductVariantID),
		OnHand:           availability.OnHand,
		Reserved:         availability.Reserved,
		Available:        availability.Available,
	}
}

func optionalInt(value *uint) *int {
	if value == nil {
		return nil
	}
	copied := int(*value)
	return &copied
}

func parseOptionalLimit(value string, fallback int, maximum int) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit < 1 {
		return fallback
	}
	if limit > maximum {
		return maximum
	}
	return limit
}
