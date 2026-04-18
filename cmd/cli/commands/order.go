package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type orderSnapshotInspect struct {
	Record          models.OrderCheckoutSnapshot       `json:"record"`
	Items           []models.OrderCheckoutSnapshotItem `json:"items"`
	TaxLines        []models.OrderTaxLine              `json:"tax_lines"`
	PaymentData     any                                `json:"payment_data,omitempty"`
	ShippingData    any                                `json:"shipping_data,omitempty"`
	TaxData         any                                `json:"tax_data,omitempty"`
	PaymentDataRaw  string                             `json:"payment_data_raw,omitempty"`
	ShippingDataRaw string                             `json:"shipping_data_raw,omitempty"`
	TaxDataRaw      string                             `json:"tax_data_raw,omitempty"`
}

type shipmentInspect struct {
	Record         models.Shipment          `json:"record"`
	Rates          []models.ShipmentRate    `json:"rates"`
	Packages       []models.ShipmentPackage `json:"packages"`
	TrackingEvents []models.TrackingEvent   `json:"tracking_events"`
}

type orderInspectResponse struct {
	Order           apicontract.Order              `json:"order"`
	User            *models.User                   `json:"user,omitempty"`
	CheckoutSession *models.CheckoutSession        `json:"checkout_session,omitempty"`
	Payments        apicontract.OrderPaymentLedger `json:"payments"`
	StatusHistory   []models.OrderStatusHistory    `json:"status_history"`
	Snapshots       []orderSnapshotInspect         `json:"snapshots"`
	Shipments       []shipmentInspect              `json:"shipments"`
}

func NewOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Order observability commands",
	}

	cmd.AddCommand(newListOrdersCmd())
	cmd.AddCommand(newGetOrderCmd())
	cmd.AddCommand(newGetOrderPaymentsCmd())
	cmd.AddCommand(newInspectOrderCmd())

	return cmd
}

func newListOrdersCmd() *cobra.Command {
	var format string
	var query string
	var page int
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			values := url.Values{}
			if strings.TrimSpace(query) != "" {
				values.Set("q", strings.TrimSpace(query))
			}
			if page > 0 {
				values.Set("page", fmt.Sprintf("%d", page))
			}
			if limit > 0 {
				values.Set("limit", fmt.Sprintf("%d", limit))
			}

			path := "/api/v1/admin/orders"
			if encoded := values.Encode(); encoded != "" {
				path += "?" + encoded
			}

			resp, err := invokeWithMediaService[apicontract.OrderPage](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.GetAllOrders(mediaService.DB, mediaService)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(resp)
				return nil
			}

			if len(resp.Data) == 0 {
				fmt.Println("No orders found")
				return nil
			}

			fmt.Printf("%-8s %-10s %-12s %-10s %-22s\n", "ID", "User", "Status", "Total", "Created")
			fmt.Println("------------------------------------------------------------------")
			for _, order := range resp.Data {
				userID := "guest"
				if order.UserId != nil {
					userID = fmt.Sprintf("%d", *order.UserId)
				}
				fmt.Printf("%-8d %-10s %-12s $%-9.2f %-22s\n",
					order.Id,
					userID,
					order.Status,
					order.Total,
					order.CreatedAt.Format(time.RFC3339),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "q", "", "Search term")
	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newGetOrderCmd() *cobra.Command {
	var id uint
	var format string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single order",
		RunE: func(cmd *cobra.Command, args []string) error {
			order, err := invokeWithMediaService[apicontract.Order](localHandlerRequest{
				Method:     http.MethodGet,
				Path:       fmt.Sprintf("/api/v1/admin/orders/%d", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
			}, func(mediaService *media.Service) gin.HandlerFunc {
				return handlers.GetAdminOrderByID(mediaService.DB, mediaService)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(order)
				return nil
			}

			fmt.Printf("Order %d\n", order.Id)
			fmt.Printf("Status: %s\n", order.Status)
			fmt.Printf("Total: $%.2f\n", order.Total)
			fmt.Printf("Items: %d\n", len(order.Items))
			fmt.Printf("Checkout Session: %d\n", order.CheckoutSessionId)
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Order ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatJSON))
	cmd.MarkFlagRequired("id")
	return cmd
}

func newGetOrderPaymentsCmd() *cobra.Command {
	var id uint
	var format string

	cmd := &cobra.Command{
		Use:   "payments",
		Short: "Get payment ledger for an order",
		RunE: func(cmd *cobra.Command, args []string) error {
			ledger, err := invokeWithDB[apicontract.OrderPaymentLedger](localHandlerRequest{
				Method:     http.MethodGet,
				Path:       fmt.Sprintf("/api/v1/admin/orders/%d/payments", id),
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.GetAdminOrderPayments(db)
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(ledger)
				return nil
			}

			if len(ledger.Intents) == 0 {
				fmt.Println("No payment intents found")
				return nil
			}
			for _, intent := range ledger.Intents {
				fmt.Printf("Intent %d provider=%s status=%s authorized=%.2f captured=%.2f refundable=%.2f\n",
					intent.Id,
					intent.Provider,
					intent.Status,
					intent.AuthorizedAmount,
					intent.CapturedAmount,
					intent.RefundableAmount,
				)
			}
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Order ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatJSON))
	cmd.MarkFlagRequired("id")
	return cmd
}

func newInspectOrderCmd() *cobra.Command {
	var id uint
	var format string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Get a deep observability view for an order",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isRemoteMode() {
				return errors.New("order inspect is only available in local path mode because it reads internal database state that is not exposed by the admin API")
			}

			mediaService := newMediaService()
			defer closeMediaService(mediaService)

			response, err := buildOrderInspectResponse(mediaService.DB, mediaService, id)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(response)
				return nil
			}

			fmt.Printf("Order %d\n", response.Order.Id)
			fmt.Printf("Status: %s\n", response.Order.Status)
			fmt.Printf("Payments: %d intent(s)\n", len(response.Payments.Intents))
			fmt.Printf("Snapshots: %d\n", len(response.Snapshots))
			fmt.Printf("Shipments: %d\n", len(response.Shipments))
			fmt.Printf("Status Events: %d\n", len(response.StatusHistory))
			return nil
		},
	}

	cmd.Flags().UintVar(&id, "id", 0, "Order ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatJSON))
	cmd.MarkFlagRequired("id")
	return cmd
}

func buildOrderInspectResponse(db *gorm.DB, mediaService *media.Service, orderID uint) (orderInspectResponse, error) {
	order, err := invokeLocalJSON[apicontract.Order](handlers.GetAdminOrderByID(db, mediaService), localHandlerRequest{
		Method:     http.MethodGet,
		Path:       fmt.Sprintf("/api/v1/admin/orders/%d", orderID),
		PathParams: map[string]string{"id": fmt.Sprintf("%d", orderID)},
	})
	if err != nil {
		return orderInspectResponse{}, err
	}

	payments, err := invokeLocalJSON[apicontract.OrderPaymentLedger](handlers.GetAdminOrderPayments(db), localHandlerRequest{
		Method:     http.MethodGet,
		Path:       fmt.Sprintf("/api/v1/admin/orders/%d/payments", orderID),
		PathParams: map[string]string{"id": fmt.Sprintf("%d", orderID)},
	})
	if err != nil {
		return orderInspectResponse{}, err
	}

	var orderRow models.Order
	if err := db.First(&orderRow, orderID).Error; err != nil {
		return orderInspectResponse{}, err
	}

	response := orderInspectResponse{
		Order:    order,
		Payments: payments,
	}

	if orderRow.UserID != nil {
		var user models.User
		if err := db.First(&user, *orderRow.UserID).Error; err == nil {
			response.User = &user
		}
	}

	var session models.CheckoutSession
	if err := db.First(&session, orderRow.CheckoutSessionID).Error; err == nil {
		response.CheckoutSession = &session
	}

	var history []models.OrderStatusHistory
	if err := db.Where("order_id = ?", orderID).Order("created_at asc").Order("id asc").Find(&history).Error; err != nil {
		return orderInspectResponse{}, err
	}
	response.StatusHistory = history

	snapshots, err := loadOrderSnapshots(db, orderID)
	if err != nil {
		return orderInspectResponse{}, err
	}
	response.Snapshots = snapshots

	shipments, err := loadOrderShipments(db, orderID)
	if err != nil {
		return orderInspectResponse{}, err
	}
	response.Shipments = shipments

	return response, nil
}

func loadOrderSnapshots(db *gorm.DB, orderID uint) ([]orderSnapshotInspect, error) {
	var snapshots []models.OrderCheckoutSnapshot
	if err := db.
		Preload("Items", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Where("order_id = ?", orderID).
		Order("created_at asc").
		Order("id asc").
		Find(&snapshots).Error; err != nil {
		return nil, err
	}

	result := make([]orderSnapshotInspect, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var taxLines []models.OrderTaxLine
		if err := db.Where("snapshot_id = ?", snapshot.ID).Order("id asc").Find(&taxLines).Error; err != nil {
			return nil, err
		}
		result = append(result, orderSnapshotInspect{
			Record:          snapshot,
			Items:           append([]models.OrderCheckoutSnapshotItem(nil), snapshot.Items...),
			TaxLines:        taxLines,
			PaymentData:     decodeLooseJSON(snapshot.PaymentDataJSON),
			ShippingData:    decodeLooseJSON(snapshot.ShippingDataJSON),
			TaxData:         decodeLooseJSON(snapshot.TaxDataJSON),
			PaymentDataRaw:  snapshot.PaymentDataJSON,
			ShippingDataRaw: snapshot.ShippingDataJSON,
			TaxDataRaw:      snapshot.TaxDataJSON,
		})
	}
	return result, nil
}

func loadOrderShipments(db *gorm.DB, orderID uint) ([]shipmentInspect, error) {
	var shipments []models.Shipment
	if err := db.
		Preload("Rates", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Preload("Packages", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Preload("TrackingEvents", func(tx *gorm.DB) *gorm.DB { return tx.Order("occurred_at asc").Order("id asc") }).
		Where("order_id = ?", orderID).
		Order("created_at asc").
		Order("id asc").
		Find(&shipments).Error; err != nil {
		return nil, err
	}

	result := make([]shipmentInspect, 0, len(shipments))
	for _, shipment := range shipments {
		result = append(result, shipmentInspect{
			Record:         shipment,
			Rates:          append([]models.ShipmentRate(nil), shipment.Rates...),
			Packages:       append([]models.ShipmentPackage(nil), shipment.Packages...),
			TrackingEvents: append([]models.TrackingEvent(nil), shipment.TrackingEvents...),
		})
	}
	return result, nil
}

func decodeLooseJSON(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	return decoded
}
