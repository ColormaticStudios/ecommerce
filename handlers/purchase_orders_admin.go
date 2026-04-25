package handlers

import (
	"net/http"
	"strconv"

	"ecommerce/internal/apicontract"
	inventoryservice "ecommerce/internal/services/inventory"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ListAdminPurchaseOrders(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orders, err := inventoryservice.ListPurchaseOrders(db, parseOptionalLimit(c.Query("limit"), 100, 200))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load purchase orders"})
			return
		}
		items := make([]apicontract.PurchaseOrder, 0, len(orders))
		for _, order := range orders {
			items = append(items, purchaseOrderContract(order))
		}
		c.JSON(http.StatusOK, apicontract.PurchaseOrderList{Items: items})
	}
}

func CreateAdminPurchaseOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request apicontract.PurchaseOrderRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase order request"})
			return
		}
		order, err := inventoryservice.CreatePurchaseOrder(db, purchaseOrderInput(request))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, purchaseOrderContract(order))
	}
}

func IssueAdminPurchaseOrder(db *gorm.DB) gin.HandlerFunc {
	return purchaseOrderTransition(db, inventoryservice.IssuePurchaseOrder)
}

func CancelAdminPurchaseOrder(db *gorm.DB) gin.HandlerFunc {
	return purchaseOrderTransition(db, inventoryservice.CancelPurchaseOrder)
}

func ReceiveAdminPurchaseOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase order id"})
			return
		}
		var request apicontract.PurchaseOrderReceiveRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid receipt request"})
			return
		}
		receipt, order, err := inventoryservice.ReceivePurchaseOrder(db, id, receivePurchaseOrderInput(request))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, apicontract.PurchaseOrderReceiptResponse{
			PurchaseOrder: purchaseOrderContract(order),
			Receipt:       receiptContract(receipt),
		})
	}
}

func purchaseOrderTransition(db *gorm.DB, transition func(*gorm.DB, uint) (models.PurchaseOrder, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositivePathID(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase order id"})
			return
		}
		order, err := transition(db, id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, purchaseOrderContract(order))
	}
}

func purchaseOrderInput(request apicontract.PurchaseOrderRequest) inventoryservice.PurchaseOrderInput {
	items := make([]inventoryservice.PurchaseOrderItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		unitCost := float64(0)
		if item.UnitCost != nil {
			unitCost = float64(*item.UnitCost)
		}
		items = append(items, inventoryservice.PurchaseOrderItemInput{
			ProductVariantID: uint(item.ProductVariantId),
			QuantityOrdered:  item.QuantityOrdered,
			UnitCost:         unitCost,
		})
	}
	var supplierID *uint
	if request.SupplierId != nil {
		id := uint(*request.SupplierId)
		supplierID = &id
	}
	var supplier *inventoryservice.SupplierInput
	if request.Supplier != nil {
		supplier = &inventoryservice.SupplierInput{
			Name:  request.Supplier.Name,
			Email: derefString(request.Supplier.Email),
			Notes: derefString(request.Supplier.Notes),
		}
	}
	return inventoryservice.PurchaseOrderInput{
		SupplierID: supplierID,
		Supplier:   supplier,
		Notes:      derefString(request.Notes),
		Items:      items,
	}
}

func receivePurchaseOrderInput(request apicontract.PurchaseOrderReceiveRequest) inventoryservice.ReceivePurchaseOrderInput {
	items := make([]inventoryservice.ReceiveItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, inventoryservice.ReceiveItemInput{
			PurchaseOrderItemID: uint(item.PurchaseOrderItemId),
			QuantityReceived:    item.QuantityReceived,
		})
	}
	return inventoryservice.ReceivePurchaseOrderInput{
		Items:     items,
		Notes:     derefString(request.Notes),
		ActorType: "admin",
	}
}

func purchaseOrderContract(order models.PurchaseOrder) apicontract.PurchaseOrder {
	items := make([]apicontract.PurchaseOrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, apicontract.PurchaseOrderItem{
			Id:               int(item.ID),
			ProductVariantId: int(item.ProductVariantID),
			QuantityOrdered:  item.QuantityOrdered,
			QuantityReceived: item.QuantityReceived,
			UnitCost:         float32(item.UnitCost.Float64()),
		})
	}
	var supplier *apicontract.Supplier
	if order.Supplier != nil {
		supplier = &apicontract.Supplier{
			Id:        int(order.Supplier.ID),
			Name:      order.Supplier.Name,
			Email:     order.Supplier.Email,
			Notes:     order.Supplier.Notes,
			CreatedAt: order.Supplier.CreatedAt,
			UpdatedAt: order.Supplier.UpdatedAt,
		}
	}
	return apicontract.PurchaseOrder{
		Id:          int(order.ID),
		SupplierId:  optionalInt(order.SupplierID),
		Supplier:    supplier,
		Status:      apicontract.PurchaseOrderStatus(order.Status),
		Notes:       order.Notes,
		IssuedAt:    order.IssuedAt,
		ReceivedAt:  order.ReceivedAt,
		CancelledAt: order.CancelledAt,
		Items:       items,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
	}
}

func receiptContract(receipt models.InventoryReceipt) apicontract.InventoryReceipt {
	items := make([]apicontract.InventoryReceiptItem, 0, len(receipt.Items))
	for _, item := range receipt.Items {
		items = append(items, apicontract.InventoryReceiptItem{
			Id:                  int(item.ID),
			PurchaseOrderItemId: int(item.PurchaseOrderItemID),
			ProductVariantId:    int(item.ProductVariantID),
			QuantityReceived:    item.QuantityReceived,
		})
	}
	return apicontract.InventoryReceipt{
		Id:              int(receipt.ID),
		PurchaseOrderId: int(receipt.PurchaseOrderID),
		ReceivedAt:      receipt.ReceivedAt,
		Notes:           receipt.Notes,
		Items:           items,
	}
}

func parsePositivePathID(c *gin.Context, name string) (uint, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id < 1 {
		return 0, err
	}
	return uint(id), nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
