package httpapi

import (
	"context"
	"errors"
	"time"

	"ecommerce/internal/apicontract"
	inventoryservice "ecommerce/internal/services/inventory"
	"ecommerce/models"
)

func (e *CatalogEndpoints) ListAdminInventoryReservations(ctx context.Context, request apicontract.ListAdminInventoryReservationsRequestObject) (apicontract.ListAdminInventoryReservationsResponseObject, error) {
	statuses := []string{}
	if request.Params.Status != nil {
		for _, value := range *request.Params.Status {
			statuses = append(statuses, string(value))
		}
	}
	values, err := e.inventory.ListReservations(ctx, statuses, limitValue(request.Params.Limit, 100, 200))
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.InventoryReservation, 0, len(values))
	for _, value := range values {
		items = append(items, inventoryReservationContract(value))
	}
	return apicontract.ListAdminInventoryReservations200JSONResponse{Items: items}, nil
}

func (e *CatalogEndpoints) ListAdminInventoryAlerts(ctx context.Context, request apicontract.ListAdminInventoryAlertsRequestObject) (apicontract.ListAdminInventoryAlertsResponseObject, error) {
	statuses := []string{}
	if request.Params.Status != nil {
		for _, value := range *request.Params.Status {
			statuses = append(statuses, string(value))
		}
	}
	values, err := e.inventory.ListAlerts(ctx, statuses, limitValue(request.Params.Limit, 100, 200))
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.InventoryAlert, 0, len(values))
	for _, value := range values {
		items = append(items, inventoryAlertContract(value))
	}
	return apicontract.ListAdminInventoryAlerts200JSONResponse{Items: items}, nil
}

func (e *CatalogEndpoints) AckAdminInventoryAlert(ctx context.Context, request apicontract.AckAdminInventoryAlertRequestObject) (apicontract.AckAdminInventoryAlertResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("inventory alert id must be positive")
	}
	value, err := e.inventory.AckAlert(ctx, uint(request.Id), inventoryservice.AlertActionInput{ActorType: "admin"})
	if err != nil {
		return nil, err
	}
	return apicontract.AckAdminInventoryAlert200JSONResponse(inventoryAlertContract(value)), nil
}

func (e *CatalogEndpoints) ResolveAdminInventoryAlert(ctx context.Context, request apicontract.ResolveAdminInventoryAlertRequestObject) (apicontract.ResolveAdminInventoryAlertResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("inventory alert id must be positive")
	}
	value, err := e.inventory.ResolveAlert(ctx, uint(request.Id), inventoryservice.AlertActionInput{ActorType: "admin"})
	if err != nil {
		return nil, err
	}
	return apicontract.ResolveAdminInventoryAlert200JSONResponse(inventoryAlertContract(value)), nil
}

func (e *CatalogEndpoints) ListAdminInventoryThresholds(ctx context.Context, request apicontract.ListAdminInventoryThresholdsRequestObject) (apicontract.ListAdminInventoryThresholdsResponseObject, error) {
	var variantID *uint
	if request.Params.ProductVariantId != nil {
		if *request.Params.ProductVariantId < 1 {
			return nil, errors.New("product variant id must be positive")
		}
		value := uint(*request.Params.ProductVariantId)
		variantID = &value
	}
	values, err := e.inventory.GetThresholds(ctx, variantID)
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.InventoryThreshold, 0, len(values))
	for _, value := range values {
		items = append(items, inventoryThresholdContract(value))
	}
	return apicontract.ListAdminInventoryThresholds200JSONResponse{Items: items}, nil
}

func (e *CatalogEndpoints) UpsertAdminInventoryThreshold(ctx context.Context, request apicontract.UpsertAdminInventoryThresholdRequestObject) (apicontract.UpsertAdminInventoryThresholdResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("inventory threshold body is required")
	}
	var variantID *uint
	if request.Body.ProductVariantId != nil {
		if *request.Body.ProductVariantId < 1 {
			return nil, errors.New("product variant id must be positive")
		}
		value := uint(*request.Body.ProductVariantId)
		variantID = &value
	}
	value, err := e.inventory.SetThreshold(ctx, inventoryservice.ThresholdInput{ProductVariantID: variantID, LowStockQuantity: request.Body.LowStockQuantity})
	if err != nil {
		return nil, err
	}
	return apicontract.UpsertAdminInventoryThreshold200JSONResponse(inventoryThresholdContract(value)), nil
}

func (e *CatalogEndpoints) DeleteAdminInventoryThreshold(ctx context.Context, request apicontract.DeleteAdminInventoryThresholdRequestObject) (apicontract.DeleteAdminInventoryThresholdResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("inventory threshold id must be positive")
	}
	if err := e.inventory.DeleteThreshold(ctx, uint(request.Id)); err != nil {
		return nil, err
	}
	return apicontract.DeleteAdminInventoryThreshold200JSONResponse{Message: "Inventory threshold deleted"}, nil
}

func (e *CatalogEndpoints) CreateAdminInventoryAdjustment(ctx context.Context, request apicontract.CreateAdminInventoryAdjustmentRequestObject) (apicontract.CreateAdminInventoryAdjustmentResponseObject, error) {
	if request.Body == nil || request.Body.ProductVariantId < 1 {
		return nil, errors.New("valid inventory adjustment body is required")
	}
	input := inventoryservice.AdjustmentInput{ProductVariantID: uint(request.Body.ProductVariantId), QuantityDelta: request.Body.QuantityDelta, ReasonCode: string(request.Body.ReasonCode), ActorType: "admin"}
	if request.Body.Notes != nil {
		input.Notes = *request.Body.Notes
	}
	if request.Body.ApprovedByType != nil {
		input.ApprovedByType = *request.Body.ApprovedByType
	}
	if request.Body.ApprovedById != nil {
		value := uint(*request.Body.ApprovedById)
		input.ApprovedByID = &value
	}
	adjustment, availability, err := e.inventory.CreateAdjustment(ctx, input, inventoryservice.AdjustmentPolicy{})
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminInventoryAdjustment201JSONResponse{Adjustment: inventoryAdjustmentContract(adjustment), Availability: inventoryAvailabilityContract(availability)}, nil
}

func (e *CatalogEndpoints) RunAdminInventoryReconciliation(ctx context.Context, _ apicontract.RunAdminInventoryReconciliationRequestObject) (apicontract.RunAdminInventoryReconciliationResponseObject, error) {
	report, err := e.inventory.Reconcile(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	issues := make([]apicontract.InventoryReconciliationIssue, 0, len(report.Issues))
	for _, value := range report.Issues {
		issues = append(issues, apicontract.InventoryReconciliationIssue{IssueType: value.IssueType, InventoryItemId: int(value.InventoryItemID), ProductVariantId: int(value.ProductVariantID), Expected: value.Expected, Actual: value.Actual, Message: value.Message, EntityType: value.EntityType, EntityId: optionalUint(value.EntityID)})
	}
	return apicontract.RunAdminInventoryReconciliation200JSONResponse{CheckedAt: report.CheckedAt, Issues: issues}, nil
}

func (e *CatalogEndpoints) GetAdminInventoryTimeline(ctx context.Context, request apicontract.GetAdminInventoryTimelineRequestObject) (apicontract.GetAdminInventoryTimelineResponseObject, error) {
	if request.ProductVariantId < 1 {
		return nil, errors.New("product variant id must be positive")
	}
	value, err := e.inventory.GetTimeline(ctx, uint(request.ProductVariantId), limitValue(request.Params.Limit, 50, 200))
	if err != nil {
		return nil, err
	}
	movements := make([]apicontract.InventoryMovement, 0, len(value.Movements))
	for _, item := range value.Movements {
		movements = append(movements, inventoryMovementContract(item))
	}
	reservations := make([]apicontract.InventoryReservation, 0, len(value.Reservations))
	for _, item := range value.Reservations {
		reservations = append(reservations, inventoryReservationContract(item))
	}
	adjustments := make([]apicontract.InventoryAdjustment, 0, len(value.Adjustments))
	for _, item := range value.Adjustments {
		adjustments = append(adjustments, inventoryAdjustmentContract(item))
	}
	return apicontract.GetAdminInventoryTimeline200JSONResponse{ProductVariantId: request.ProductVariantId, Movements: movements, Reservations: reservations, Adjustments: adjustments}, nil
}

func (e *CatalogEndpoints) ListAdminPurchaseOrders(ctx context.Context, request apicontract.ListAdminPurchaseOrdersRequestObject) (apicontract.ListAdminPurchaseOrdersResponseObject, error) {
	values, err := e.inventory.ListPurchaseOrders(ctx, limitValue(request.Params.Limit, 100, 200))
	if err != nil {
		return nil, err
	}
	items := make([]apicontract.PurchaseOrder, 0, len(values))
	for _, value := range values {
		items = append(items, purchaseOrderContract(value))
	}
	return apicontract.ListAdminPurchaseOrders200JSONResponse{Items: items}, nil
}

func (e *CatalogEndpoints) CreateAdminPurchaseOrder(ctx context.Context, request apicontract.CreateAdminPurchaseOrderRequestObject) (apicontract.CreateAdminPurchaseOrderResponseObject, error) {
	if request.Body == nil {
		return nil, errors.New("purchase order body is required")
	}
	value, err := e.inventory.CreatePurchaseOrder(ctx, purchaseOrderInput(*request.Body))
	if err != nil {
		return nil, err
	}
	return apicontract.CreateAdminPurchaseOrder201JSONResponse(purchaseOrderContract(value)), nil
}

func (e *CatalogEndpoints) IssueAdminPurchaseOrder(ctx context.Context, request apicontract.IssueAdminPurchaseOrderRequestObject) (apicontract.IssueAdminPurchaseOrderResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("purchase order id must be positive")
	}
	value, err := e.inventory.IssuePurchaseOrder(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.IssueAdminPurchaseOrder200JSONResponse(purchaseOrderContract(value)), nil
}
func (e *CatalogEndpoints) CancelAdminPurchaseOrder(ctx context.Context, request apicontract.CancelAdminPurchaseOrderRequestObject) (apicontract.CancelAdminPurchaseOrderResponseObject, error) {
	if request.Id < 1 {
		return nil, errors.New("purchase order id must be positive")
	}
	value, err := e.inventory.CancelPurchaseOrder(ctx, uint(request.Id))
	if err != nil {
		return nil, err
	}
	return apicontract.CancelAdminPurchaseOrder200JSONResponse(purchaseOrderContract(value)), nil
}
func (e *CatalogEndpoints) ReceiveAdminPurchaseOrder(ctx context.Context, request apicontract.ReceiveAdminPurchaseOrderRequestObject) (apicontract.ReceiveAdminPurchaseOrderResponseObject, error) {
	if request.Id < 1 || request.Body == nil {
		return nil, errors.New("valid purchase order id and receipt body are required")
	}
	input := inventoryservice.ReceivePurchaseOrderInput{Notes: derefString(request.Body.Notes), ActorType: "admin"}
	for _, item := range request.Body.Items {
		input.Items = append(input.Items, inventoryservice.ReceiveItemInput{PurchaseOrderItemID: uint(item.PurchaseOrderItemId), QuantityReceived: item.QuantityReceived})
	}
	receipt, order, err := e.inventory.ReceivePurchaseOrder(ctx, uint(request.Id), input)
	if err != nil {
		return nil, err
	}
	return apicontract.ReceiveAdminPurchaseOrder200JSONResponse{PurchaseOrder: purchaseOrderContract(order), Receipt: receiptContract(receipt)}, nil
}

func limitValue(value *int, fallback, maximum int) int {
	if value == nil || *value < 1 {
		return fallback
	}
	return min(*value, maximum)
}
func optionalUint(value *uint) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func inventoryReservationContract(value models.InventoryReservation) apicontract.InventoryReservation {
	return apicontract.InventoryReservation{Id: int(value.ID), ProductVariantId: int(value.ProductVariantID), Quantity: value.Quantity, Status: apicontract.InventoryReservationStatus(value.Status), ExpiresAt: value.ExpiresAt, OwnerType: value.OwnerType, OwnerId: optionalUint(value.OwnerID), CheckoutSessionId: optionalUint(value.CheckoutSessionID), OrderId: optionalUint(value.OrderID), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func inventoryAlertContract(value models.InventoryAlert) apicontract.InventoryAlert {
	return apicontract.InventoryAlert{Id: int(value.ID), ProductVariantId: int(value.ProductVariantID), AlertType: apicontract.InventoryAlertAlertType(value.AlertType), Status: apicontract.InventoryAlertStatus(value.Status), Available: value.Available, Threshold: value.Threshold, OpenedAt: value.OpenedAt, AckedAt: value.AckedAt, AckedByType: optionalString(value.AckedByType), AckedById: optionalUint(value.AckedByID), ResolvedAt: value.ResolvedAt, ResolvedByType: optionalString(value.ResolvedByType), ResolvedById: optionalUint(value.ResolvedByID), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func inventoryThresholdContract(value models.InventoryThreshold) apicontract.InventoryThreshold {
	return apicontract.InventoryThreshold{Id: int(value.ID), ProductVariantId: optionalUint(value.ProductVariantID), LowStockQuantity: value.LowStockQuantity, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func inventoryAdjustmentContract(value models.InventoryAdjustment) apicontract.InventoryAdjustment {
	return apicontract.InventoryAdjustment{Id: int(value.ID), InventoryItemId: int(value.InventoryItemID), ProductVariantId: int(value.ProductVariantID), QuantityDelta: value.QuantityDelta, ReasonCode: apicontract.InventoryAdjustmentReason(value.ReasonCode), Notes: value.Notes, ActorType: value.ActorType, ActorId: optionalUint(value.ActorID), ApprovedByType: value.ApprovedByType, ApprovedById: optionalUint(value.ApprovedByID), ApprovedAt: value.ApprovedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func inventoryAvailabilityContract(value inventoryservice.Availability) apicontract.InventoryAvailability {
	return apicontract.InventoryAvailability{ProductVariantId: int(value.ProductVariantID), OnHand: value.OnHand, Reserved: value.Reserved, Available: value.Available}
}
func inventoryMovementContract(value models.InventoryMovement) apicontract.InventoryMovement {
	return apicontract.InventoryMovement{Id: int(value.ID), InventoryItemId: int(value.InventoryItemID), MovementType: value.MovementType, QuantityDelta: value.QuantityDelta, ReferenceType: value.ReferenceType, ReferenceId: optionalUint(value.ReferenceID), ReasonCode: value.ReasonCode, ActorType: value.ActorType, ActorId: optionalUint(value.ActorID), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func purchaseOrderInput(request apicontract.PurchaseOrderRequest) inventoryservice.PurchaseOrderInput {
	input := inventoryservice.PurchaseOrderInput{Notes: derefString(request.Notes)}
	if request.SupplierId != nil {
		value := uint(*request.SupplierId)
		input.SupplierID = &value
	}
	if request.Supplier != nil {
		input.Supplier = &inventoryservice.SupplierInput{Name: request.Supplier.Name, Email: derefString(request.Supplier.Email), Notes: derefString(request.Supplier.Notes)}
	}
	for _, item := range request.Items {
		cost := 0.0
		if item.UnitCost != nil {
			cost = float64(*item.UnitCost)
		}
		input.Items = append(input.Items, inventoryservice.PurchaseOrderItemInput{ProductVariantID: uint(item.ProductVariantId), QuantityOrdered: item.QuantityOrdered, UnitCost: cost})
	}
	return input
}
func purchaseOrderContract(value models.PurchaseOrder) apicontract.PurchaseOrder {
	items := make([]apicontract.PurchaseOrderItem, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, apicontract.PurchaseOrderItem{Id: int(item.ID), ProductVariantId: int(item.ProductVariantID), QuantityOrdered: item.QuantityOrdered, QuantityReceived: item.QuantityReceived, UnitCost: float32(item.UnitCost.Float64())})
	}
	var supplier *apicontract.Supplier
	if value.Supplier != nil {
		supplier = &apicontract.Supplier{Id: int(value.Supplier.ID), Name: value.Supplier.Name, Email: value.Supplier.Email, Notes: value.Supplier.Notes, CreatedAt: value.Supplier.CreatedAt, UpdatedAt: value.Supplier.UpdatedAt}
	}
	return apicontract.PurchaseOrder{Id: int(value.ID), SupplierId: optionalUint(value.SupplierID), Supplier: supplier, Status: apicontract.PurchaseOrderStatus(value.Status), Notes: value.Notes, IssuedAt: value.IssuedAt, ReceivedAt: value.ReceivedAt, CancelledAt: value.CancelledAt, Items: items, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func receiptContract(value models.InventoryReceipt) apicontract.InventoryReceipt {
	items := make([]apicontract.InventoryReceiptItem, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, apicontract.InventoryReceiptItem{Id: int(item.ID), PurchaseOrderItemId: int(item.PurchaseOrderItemID), ProductVariantId: int(item.ProductVariantID), QuantityReceived: item.QuantityReceived})
	}
	return apicontract.InventoryReceipt{Id: int(value.ID), PurchaseOrderId: int(value.PurchaseOrderID), ReceivedAt: value.ReceivedAt, Notes: value.Notes, Items: items}
}
