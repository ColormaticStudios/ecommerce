package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CheckoutOrderShippingRatesRequest struct {
	SnapshotID uint `json:"snapshot_id" binding:"required"`
}

type AdminOrderShippingLabelPackageRequest struct {
	Reference   string `json:"reference"`
	WeightGrams int    `json:"weight_grams"`
	LengthCM    int    `json:"length_cm"`
	WidthCM     int    `json:"width_cm"`
	HeightCM    int    `json:"height_cm"`
}

type AdminOrderShippingLabelRequest struct {
	RateID  uint                                   `json:"rate_id" binding:"required"`
	Package *AdminOrderShippingLabelPackageRequest `json:"package,omitempty"`
}

type CheckoutOrderTaxFinalizeRequest struct {
	SnapshotID       uint  `json:"snapshot_id" binding:"required"`
	InclusivePricing *bool `json:"inclusive_pricing,omitempty"`
}

type shipmentRateResponse struct {
	ID             uint    `json:"id"`
	Provider       string  `json:"provider"`
	ProviderRateID string  `json:"provider_rate_id"`
	ServiceCode    string  `json:"service_code"`
	ServiceName    string  `json:"service_name"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Selected       bool    `json:"selected"`
	ShipmentID     *uint   `json:"shipment_id,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
}

type shipmentPackageResponse struct {
	ID          uint   `json:"id"`
	Reference   string `json:"reference"`
	WeightGrams int    `json:"weight_grams"`
	LengthCM    int    `json:"length_cm"`
	WidthCM     int    `json:"width_cm"`
	HeightCM    int    `json:"height_cm"`
}

type trackingEventResponse struct {
	ID              uint   `json:"id"`
	Provider        string `json:"provider"`
	ProviderEventID string `json:"provider_event_id"`
	Status          string `json:"status"`
	TrackingNumber  string `json:"tracking_number"`
	Location        string `json:"location"`
	Description     string `json:"description"`
	OccurredAt      string `json:"occurred_at"`
}

type shipmentResponse struct {
	ID                    uint                      `json:"id"`
	OrderID               uint                      `json:"order_id"`
	SnapshotID            uint                      `json:"snapshot_id"`
	Provider              string                    `json:"provider"`
	ShipmentRateID        uint                      `json:"shipment_rate_id"`
	ProviderShipmentID    string                    `json:"provider_shipment_id"`
	Status                string                    `json:"status"`
	Currency              string                    `json:"currency"`
	ServiceCode           string                    `json:"service_code"`
	ServiceName           string                    `json:"service_name"`
	Amount                float64                   `json:"amount"`
	ShippingAddressPretty string                    `json:"shipping_address_pretty"`
	TrackingNumber        string                    `json:"tracking_number"`
	TrackingURL           string                    `json:"tracking_url"`
	LabelURL              string                    `json:"label_url"`
	PurchasedAt           *string                   `json:"purchased_at,omitempty"`
	FinalizedAt           *string                   `json:"finalized_at,omitempty"`
	DeliveredAt           *string                   `json:"delivered_at,omitempty"`
	Rates                 []shipmentRateResponse    `json:"rates"`
	Packages              []shipmentPackageResponse `json:"packages"`
	TrackingEvents        []trackingEventResponse   `json:"tracking_events"`
}

type checkoutOrderShippingRatesResponse struct {
	OrderID    uint                   `json:"order_id"`
	SnapshotID uint                   `json:"snapshot_id"`
	Provider   string                 `json:"provider"`
	Rates      []shipmentRateResponse `json:"rates"`
}

type checkoutOrderTrackingResponse struct {
	OrderID   uint               `json:"order_id"`
	Shipments []shipmentResponse `json:"shipments"`
}

type adminOrderShippingLabelResponse struct {
	Message  string           `json:"message"`
	Shipment shipmentResponse `json:"shipment"`
}

type taxLineResponse struct {
	ID                 uint    `json:"id,omitempty"`
	SnapshotItemID     *uint   `json:"snapshot_item_id,omitempty"`
	LineType           string  `json:"line_type"`
	ProductVariantID   *uint   `json:"product_variant_id,omitempty"`
	Quantity           int     `json:"quantity"`
	Jurisdiction       string  `json:"jurisdiction"`
	TaxCode            string  `json:"tax_code"`
	TaxName            string  `json:"tax_name"`
	TaxableAmount      float64 `json:"taxable_amount"`
	TaxAmount          float64 `json:"tax_amount"`
	TaxRateBasisPoints int     `json:"tax_rate_basis_points"`
	Inclusive          bool    `json:"inclusive"`
}

type checkoutOrderTaxFinalizeResponse struct {
	Message          string            `json:"message"`
	OrderID          uint              `json:"order_id"`
	SnapshotID       uint              `json:"snapshot_id"`
	Provider         string            `json:"provider"`
	Currency         string            `json:"currency"`
	InclusivePricing bool              `json:"inclusive_pricing"`
	TotalTax         float64           `json:"total_tax"`
	Lines            []taxLineResponse `json:"lines"`
}

func QuoteCheckoutOrderShippingRates(
	db *gorm.DB,
	providerRegistry shippingservice.ProviderRegistry,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
) gin.HandlerFunc {
	if providerRegistry == nil {
		providerRegistry = shippingservice.NewDefaultProviderRegistry()
	}
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutOrderRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		var req CheckoutOrderShippingRatesRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		scope := fmt.Sprintf("checkout_order_shipping_rates:%d:%d", orderID, req.SnapshotID)
		replayedRecord, handled, err := replayCheckoutIdempotency(db, c, requestCtx.Session, scope, req)
		if err != nil {
			_ = replayedRecord
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process shipping rate request"})
			return
		}
		if handled {
			return
		}

		idempotencyRecord, handled, err := beginCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			scope,
			req,
			checkoutCorrelationID(c, ""),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process shipping rate request"})
			return
		}
		if handled {
			return
		}

		responseWritten := false
		respond := func(status int, payload any) {
			responseWritten = true
			writeCheckoutJSON(db, c, idempotencyRecord, status, payload)
		}

		var (
			order    models.Order
			snapshot models.OrderCheckoutSnapshot
			rates    []models.ShipmentRate
		)
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("id = ? AND checkout_session_id = ?", orderID, requestCtx.Session.ID).
				Preload("Items.ProductVariant").
				First(&order).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					respond(http.StatusNotFound, gin.H{"error": "Order not found"})
					return nil
				}
				return err
			}

			snapshot, err = paymentservice.GetCheckoutSnapshotForSession(tx, requestCtx.Session.ID, req.SnapshotID)
			if err != nil {
				if errors.Is(err, paymentservice.ErrSnapshotNotFound) {
					respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot not found"})
					return nil
				}
				return err
			}
			if err := paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
				switch {
				case errors.Is(err, paymentservice.ErrSnapshotExpired):
					respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot has expired"})
					return nil
				case errors.Is(err, paymentservice.ErrSnapshotOrderMismatch):
					respond(http.StatusConflict, gin.H{"error": "Checkout snapshot no longer matches the order"})
					return nil
				case errors.Is(err, paymentservice.ErrSnapshotAlreadyBound):
					respond(http.StatusConflict, gin.H{"error": "Checkout snapshot is already bound to another order"})
					return nil
				default:
					return err
				}
			}

			rates, err = shippingservice.QuoteAndPersistRates(
				c.Request.Context(),
				tx,
				providerRegistry,
				order,
				snapshot,
				time.Now().UTC(),
			)
			if err != nil {
				switch {
				case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
					respond(http.StatusConflict, gin.H{"error": "Provider credential is not configured for this environment"})
					return nil
				case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
					respond(http.StatusBadRequest, gin.H{"error": "Provider does not support the requested currency"})
					return nil
				}
			}
			return err
		})
		if err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to quote shipping rates"})
			return
		}
		if responseWritten {
			return
		}

		respond(http.StatusOK, checkoutOrderShippingRatesResponse{
			OrderID:    order.ID,
			SnapshotID: snapshot.ID,
			Provider:   snapshot.ShippingProviderID,
			Rates:      serializeShipmentRates(rates),
		})
	}
}

func CreateAdminOrderShippingLabel(db *gorm.DB, providerRegistry shippingservice.ProviderRegistry) gin.HandlerFunc {
	if providerRegistry == nil {
		providerRegistry = shippingservice.NewDefaultProviderRegistry()
	}
	return func(c *gin.Context) {
		adminUser, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var req AdminOrderShippingLabelRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		scope := fmt.Sprintf("admin_order_shipping_label:%d:%d:%d", adminUser.ID, orderID, req.RateID)
		idempotencyRecord, handled, err := beginScopedIdempotency(db, c, scope, req, checkoutCorrelationID(c, ""))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process shipping label request"})
			return
		}
		if handled {
			return
		}

		responseWritten := false
		respond := func(status int, payload any) {
			responseWritten = true
			writeCheckoutJSON(db, c, idempotencyRecord, status, payload)
		}

		var order models.Order
		if err := db.Select("id").First(&order, orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				respond(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			}
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to create shipping label"})
			return
		}

		pkg := shippingservice.PackageInput{}
		if req.Package != nil {
			pkg = shippingservice.PackageInput{
				Reference:   req.Package.Reference,
				WeightGrams: req.Package.WeightGrams,
				LengthCM:    req.Package.LengthCM,
				WidthCM:     req.Package.WidthCM,
				HeightCM:    req.Package.HeightCM,
			}
		}

		shipment, err := shippingservice.PurchaseLabel(
			c.Request.Context(),
			db,
			providerRegistry,
			order.ID,
			req.RateID,
			pkg,
			strings.TrimSpace(c.GetHeader("Idempotency-Key")),
			checkoutCorrelationID(c, ""),
			time.Now().UTC(),
		)
		if err != nil {
			switch {
			case errors.Is(err, shippingservice.ErrShipmentRateNotFound):
				respond(http.StatusNotFound, gin.H{"error": "Shipment rate not found"})
			case errors.Is(err, shippingservice.ErrShipmentServiceImmutable):
				respond(http.StatusConflict, gin.H{"error": "Chosen shipping service is immutable for this finalized shipment"})
			case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
				respond(http.StatusConflict, gin.H{"error": "Provider credential is not configured for this environment"})
			case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
				respond(http.StatusBadRequest, gin.H{"error": "Provider does not support the requested currency"})
			default:
				respond(http.StatusInternalServerError, gin.H{"error": "Failed to create shipping label"})
			}
			return
		}
		if responseWritten {
			return
		}

		respond(http.StatusOK, adminOrderShippingLabelResponse{
			Message:  "Shipping label purchased",
			Shipment: serializeShipment(shipment),
		})
	}
}

func GetCheckoutOrderShippingTracking(
	db *gorm.DB,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutOrderRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var order models.Order
		if err := db.Select("id").Where("id = ? AND checkout_session_id = ?", orderID, requestCtx.Session.ID).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load order"})
			return
		}

		shipments, err := shippingservice.GetOrderShipments(db, order.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load shipment tracking"})
			return
		}

		response := checkoutOrderTrackingResponse{
			OrderID:   order.ID,
			Shipments: make([]shipmentResponse, 0, len(shipments)),
		}
		for _, shipment := range shipments {
			response.Shipments = append(response.Shipments, serializeShipment(shipment))
		}
		c.JSON(http.StatusOK, response)
	}
}

func FinalizeCheckoutOrderTax(
	db *gorm.DB,
	providerRegistry taxservice.ProviderRegistry,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
) gin.HandlerFunc {
	if providerRegistry == nil {
		providerRegistry = taxservice.NewDefaultProviderRegistry()
	}
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutOrderRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		var req CheckoutOrderTaxFinalizeRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		orderID, err := parseUintParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		scope := fmt.Sprintf("checkout_order_tax_finalize:%d:%d", orderID, req.SnapshotID)
		replayedRecord, handled, err := replayCheckoutIdempotency(db, c, requestCtx.Session, scope, req)
		if err != nil {
			_ = replayedRecord
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process tax finalization"})
			return
		}
		if handled {
			return
		}

		idempotencyRecord, handled, err := beginCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			scope,
			req,
			checkoutCorrelationID(c, ""),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process tax finalization"})
			return
		}
		if handled {
			return
		}

		responseWritten := false
		respond := func(status int, payload any) {
			responseWritten = true
			writeCheckoutJSON(db, c, idempotencyRecord, status, payload)
		}

		var (
			order    models.Order
			snapshot models.OrderCheckoutSnapshot
			result   taxservice.TaxFinalized
		)
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("id = ? AND checkout_session_id = ?", orderID, requestCtx.Session.ID).
				Preload("Items.ProductVariant").
				First(&order).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					respond(http.StatusNotFound, gin.H{"error": "Order not found"})
					return nil
				}
				return err
			}

			snapshot, err = paymentservice.GetCheckoutSnapshotForSession(tx, requestCtx.Session.ID, req.SnapshotID)
			if err != nil {
				if errors.Is(err, paymentservice.ErrSnapshotNotFound) {
					respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot not found"})
					return nil
				}
				return err
			}
			if err := paymentservice.ValidateSnapshotForOrder(&snapshot, &order, time.Now().UTC()); err != nil {
				switch {
				case errors.Is(err, paymentservice.ErrSnapshotExpired):
					respond(http.StatusBadRequest, gin.H{"error": "Checkout snapshot has expired"})
					return nil
				case errors.Is(err, paymentservice.ErrSnapshotOrderMismatch):
					respond(http.StatusConflict, gin.H{"error": "Checkout snapshot no longer matches the order"})
					return nil
				case errors.Is(err, paymentservice.ErrSnapshotAlreadyBound):
					respond(http.StatusConflict, gin.H{"error": "Checkout snapshot is already bound to another order"})
					return nil
				default:
					return err
				}
			}

			result, err = taxservice.FinalizeOrderTax(
				c.Request.Context(),
				tx,
				providerRegistry,
				taxservice.FinalizeInput{
					Order:            order,
					Snapshot:         snapshot,
					InclusivePricing: req.InclusivePricing,
				},
			)
			if err != nil {
				switch {
				case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
					respond(http.StatusConflict, gin.H{"error": "Provider credential is not configured for this environment"})
					return nil
				case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
					respond(http.StatusBadRequest, gin.H{"error": "Provider does not support the requested currency"})
					return nil
				}
			}
			return err
		})
		if err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to finalize taxes"})
			return
		}
		if responseWritten {
			return
		}

		respond(http.StatusOK, serializeTaxFinalizeResponse(snapshot, result))
	}
}

func ExportAdminTaxReport(db *gorm.DB, providerRegistry taxservice.ProviderRegistry) gin.HandlerFunc {
	if providerRegistry == nil {
		providerRegistry = taxservice.NewDefaultProviderRegistry()
	}
	return func(c *gin.Context) {
		format := strings.TrimSpace(c.DefaultQuery("format", "csv"))
		if format != "csv" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only csv tax export is supported"})
			return
		}

		start, err := parseOptionalTimeQuery(c.Query("start_date"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date"})
			return
		}
		end, err := parseOptionalTimeQuery(c.Query("end_date"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date"})
			return
		}

		responseWritten := false
		respond := func(status int, payload any) {
			responseWritten = true
			c.JSON(status, payload)
		}

		var (
			record models.TaxExport
			body   io.ReadCloser
		)
		err = db.Transaction(func(tx *gorm.DB) error {
			record, body, err = taxservice.ExportOrderTaxes(
				c.Request.Context(),
				tx,
				providerRegistry,
				taxservice.ExportInput{
					Provider: c.Query("provider"),
					Start:    start,
					End:      end,
					Format:   format,
				},
			)
			if err != nil {
				switch {
				case errors.Is(err, providerops.ErrProviderCredentialWrongEnvironment):
					respond(http.StatusConflict, gin.H{"error": "Provider credential is not configured for this environment"})
					return nil
				case errors.Is(err, providerops.ErrUnsupportedProviderCurrency):
					respond(http.StatusBadRequest, gin.H{"error": "Provider does not support the requested currency"})
					return nil
				}
			}
			return err
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export tax report"})
			return
		}
		if responseWritten {
			return
		}
		defer body.Close()

		content, err := io.ReadAll(body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read tax report"})
			return
		}

		filename := fmt.Sprintf("tax-report-%d.csv", record.ID)
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		c.Data(http.StatusOK, "text/csv; charset=utf-8", content)
	}
}

func serializeShipmentRates(rates []models.ShipmentRate) []shipmentRateResponse {
	response := make([]shipmentRateResponse, 0, len(rates))
	for _, rate := range rates {
		var expiresAt *string
		if rate.ExpiresAt != nil {
			value := rate.ExpiresAt.UTC().Format(timeRFC3339JSON)
			expiresAt = &value
		}
		response = append(response, shipmentRateResponse{
			ID:             rate.ID,
			Provider:       rate.Provider,
			ProviderRateID: rate.ProviderRateID,
			ServiceCode:    rate.ServiceCode,
			ServiceName:    rate.ServiceName,
			Amount:         rate.Amount.Float64(),
			Currency:       rate.Currency,
			Selected:       rate.Selected,
			ShipmentID:     rate.ShipmentID,
			ExpiresAt:      expiresAt,
		})
	}
	return response
}

func serializeShipment(shipment models.Shipment) shipmentResponse {
	response := shipmentResponse{
		ID:                    shipment.ID,
		OrderID:               shipment.OrderID,
		SnapshotID:            shipment.SnapshotID,
		Provider:              shipment.Provider,
		ShipmentRateID:        shipment.ShipmentRateID,
		ProviderShipmentID:    shipment.ProviderShipmentID,
		Status:                shipment.Status,
		Currency:              shipment.Currency,
		ServiceCode:           shipment.ServiceCode,
		ServiceName:           shipment.ServiceName,
		Amount:                shipment.Amount.Float64(),
		ShippingAddressPretty: shipment.ShippingAddressPretty,
		TrackingNumber:        shipment.TrackingNumber,
		TrackingURL:           shipment.TrackingURL,
		LabelURL:              shipment.LabelURL,
		Rates:                 serializeShipmentRates(shipment.Rates),
		Packages:              make([]shipmentPackageResponse, 0, len(shipment.Packages)),
		TrackingEvents:        make([]trackingEventResponse, 0, len(shipment.TrackingEvents)),
	}
	if shipment.PurchasedAt != nil {
		value := shipment.PurchasedAt.UTC().Format(timeRFC3339JSON)
		response.PurchasedAt = &value
	}
	if shipment.FinalizedAt != nil {
		value := shipment.FinalizedAt.UTC().Format(timeRFC3339JSON)
		response.FinalizedAt = &value
	}
	if shipment.DeliveredAt != nil {
		value := shipment.DeliveredAt.UTC().Format(timeRFC3339JSON)
		response.DeliveredAt = &value
	}
	for _, pkg := range shipment.Packages {
		response.Packages = append(response.Packages, shipmentPackageResponse{
			ID:          pkg.ID,
			Reference:   pkg.Reference,
			WeightGrams: pkg.WeightGrams,
			LengthCM:    pkg.LengthCM,
			WidthCM:     pkg.WidthCM,
			HeightCM:    pkg.HeightCM,
		})
	}
	for _, event := range shipment.TrackingEvents {
		response.TrackingEvents = append(response.TrackingEvents, trackingEventResponse{
			ID:              event.ID,
			Provider:        event.Provider,
			ProviderEventID: event.ProviderEventID,
			Status:          event.Status,
			TrackingNumber:  event.TrackingNumber,
			Location:        event.Location,
			Description:     event.Description,
			OccurredAt:      event.OccurredAt.UTC().Format(timeRFC3339JSON),
		})
	}
	return response
}

func serializeTaxFinalizeResponse(
	snapshot models.OrderCheckoutSnapshot,
	result taxservice.TaxFinalized,
) checkoutOrderTaxFinalizeResponse {
	response := checkoutOrderTaxFinalizeResponse{
		Message:          "Taxes finalized",
		OrderID:          derefUint(snapshot.OrderID),
		SnapshotID:       snapshot.ID,
		Provider:         result.Provider,
		Currency:         result.Currency,
		InclusivePricing: result.InclusivePricing,
		TotalTax:         result.TotalTax.Float64(),
		Lines:            make([]taxLineResponse, 0, len(result.Lines)),
	}
	for _, line := range result.Lines {
		response.Lines = append(response.Lines, taxLineResponse{
			SnapshotItemID:     line.SnapshotItemID,
			LineType:           line.LineType,
			ProductVariantID:   line.ProductVariantID,
			Quantity:           line.Quantity,
			Jurisdiction:       line.Jurisdiction,
			TaxCode:            line.TaxCode,
			TaxName:            line.TaxName,
			TaxableAmount:      line.TaxableAmount.Float64(),
			TaxAmount:          line.TaxAmount.Float64(),
			TaxRateBasisPoints: line.TaxRateBasisPoints,
			Inclusive:          line.Inclusive,
		})
	}
	return response
}

func parseOptionalTimeQuery(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		utc := parsed.UTC()
		return &utc, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func derefUint(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}
