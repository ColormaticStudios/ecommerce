package commands

import (
	"context"
	"fmt"
	"log"

	"net/url"
	"strconv"
	"strings"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"

	"github.com/spf13/cobra"
)

func NewInventoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Inventory discipline commands",
		Long:  "Commands for reservations, alerts, thresholds, adjustments, reconciliation, and inventory timelines.",
	}

	cmd.AddCommand(newInventoryReservationsCmd())
	cmd.AddCommand(newInventoryAlertsCmd())
	cmd.AddCommand(newInventoryThresholdsCmd())
	cmd.AddCommand(newInventoryAdjustmentCmd())
	cmd.AddCommand(newInventoryReconcileCmd())
	cmd.AddCommand(newInventoryTimelineCmd())

	return cmd
}

func newInventoryReservationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservations",
		Short: "Inventory reservation commands",
	}
	cmd.AddCommand(newInventoryReservationsListCmd())
	return cmd
}

func newInventoryReservationsListCmd() *cobra.Command {
	var statuses []string
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List inventory reservations",
		Run: func(cmd *cobra.Command, args []string) {
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			response, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryReservationList, error) {
				values := make([]apicontract.ListAdminInventoryReservationsParamsStatus, 0, len(statuses))
				for _, value := range statuses {
					values = append(values, apicontract.ListAdminInventoryReservationsParamsStatus(strings.ToUpper(value)))
				}
				result, err := endpoints.ListAdminInventoryReservations(ctx, apicontract.ListAdminInventoryReservationsRequestObject{Params: apicontract.ListAdminInventoryReservationsParams{Status: &values, Limit: &limit}})
				if err != nil {
					return apicontract.InventoryReservationList{}, err
				}
				return apicontract.InventoryReservationList(result.(apicontract.ListAdminInventoryReservations200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(response)
				return
			}
			printInventoryReservations(response.Items)
		},
	}

	cmd.Flags().StringSliceVar(&statuses, "status", nil, "Filter by reservation status (repeatable or comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum reservations to return")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Inventory alert commands",
	}
	cmd.AddCommand(newInventoryAlertsListCmd())
	cmd.AddCommand(newInventoryAlertActionCmd("ack", "Acknowledge an inventory alert"))
	cmd.AddCommand(newInventoryAlertActionCmd("resolve", "Resolve an inventory alert"))
	return cmd
}

func newInventoryAlertsListCmd() *cobra.Command {
	var statuses []string
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List inventory alerts",
		Run: func(cmd *cobra.Command, args []string) {
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			response, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryAlertList, error) {
				values := make([]apicontract.ListAdminInventoryAlertsParamsStatus, 0, len(statuses))
				for _, value := range statuses {
					values = append(values, apicontract.ListAdminInventoryAlertsParamsStatus(strings.ToUpper(value)))
				}
				result, err := endpoints.ListAdminInventoryAlerts(ctx, apicontract.ListAdminInventoryAlertsRequestObject{Params: apicontract.ListAdminInventoryAlertsParams{Status: &values, Limit: &limit}})
				if err != nil {
					return apicontract.InventoryAlertList{}, err
				}
				return apicontract.InventoryAlertList(result.(apicontract.ListAdminInventoryAlerts200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(response)
				return
			}
			printInventoryAlerts(response.Items)
		},
	}

	cmd.Flags().StringSliceVar(&statuses, "status", nil, "Filter by alert status (repeatable or comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum alerts to return")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryAlertActionCmd(name string, short string) *cobra.Command {
	var alertID int
	var format string

	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Run: func(cmd *cobra.Command, args []string) {
			if alertID < 1 {
				log.Fatal("provide --id")
			}
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			alert, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryAlert, error) {
				if name == "ack" {
					result, err := endpoints.AckAdminInventoryAlert(ctx, apicontract.AckAdminInventoryAlertRequestObject{Id: alertID})
					if err != nil {
						return apicontract.InventoryAlert{}, err
					}
					return apicontract.InventoryAlert(result.(apicontract.AckAdminInventoryAlert200JSONResponse)), nil
				}
				result, err := endpoints.ResolveAdminInventoryAlert(ctx, apicontract.ResolveAdminInventoryAlertRequestObject{Id: alertID})
				if err != nil {
					return apicontract.InventoryAlert{}, err
				}
				return apicontract.InventoryAlert(result.(apicontract.ResolveAdminInventoryAlert200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(alert)
				return
			}
			fmt.Printf("Inventory alert %d %s\n", alert.Id, inventoryAlertActionPastTense(name))
		},
	}

	cmd.Flags().IntVar(&alertID, "id", 0, "Inventory alert ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryThresholdsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thresholds",
		Short: "Inventory threshold commands",
	}
	cmd.AddCommand(newInventoryThresholdsListCmd())
	cmd.AddCommand(newInventoryThresholdSetCmd())
	cmd.AddCommand(newInventoryThresholdDeleteCmd())
	return cmd
}

func newInventoryThresholdsListCmd() *cobra.Command {
	var variantID int
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List inventory thresholds",
		Run: func(cmd *cobra.Command, args []string) {
			if variantID < 0 {
				log.Fatal("--variant-id must be positive")
			}
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			response, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryThresholdList, error) {
				var id *int
				if variantID > 0 {
					id = &variantID
				}
				result, err := endpoints.ListAdminInventoryThresholds(ctx, apicontract.ListAdminInventoryThresholdsRequestObject{Params: apicontract.ListAdminInventoryThresholdsParams{ProductVariantId: id}})
				if err != nil {
					return apicontract.InventoryThresholdList{}, err
				}
				return apicontract.InventoryThresholdList(result.(apicontract.ListAdminInventoryThresholds200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(response)
				return
			}
			printInventoryThresholds(response.Items)
		},
	}

	cmd.Flags().IntVar(&variantID, "variant-id", 0, "Filter by product variant ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryThresholdSetCmd() *cobra.Command {
	var variantID int
	var lowStock int
	var format string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update an inventory threshold",
		Run: func(cmd *cobra.Command, args []string) {
			if lowStock < 0 {
				log.Fatal("--low-stock must be zero or greater")
			}
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			var productVariantID *int
			if cmd.Flags().Changed("variant-id") {
				if variantID < 1 {
					log.Fatal("--variant-id must be positive")
				}
				productVariantID = &variantID
			}
			body := apicontract.InventoryThresholdRequest{
				ProductVariantId: productVariantID,
				LowStockQuantity: lowStock,
			}
			threshold, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryThreshold, error) {
				result, err := endpoints.UpsertAdminInventoryThreshold(ctx, apicontract.UpsertAdminInventoryThresholdRequestObject{Body: &body})
				if err != nil {
					return apicontract.InventoryThreshold{}, err
				}
				return apicontract.InventoryThreshold(result.(apicontract.UpsertAdminInventoryThreshold200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(threshold)
				return
			}
			printInventoryThresholds([]apicontract.InventoryThreshold{threshold})
		},
	}

	cmd.Flags().IntVar(&variantID, "variant-id", 0, "Product variant ID; omit for the default threshold")
	cmd.Flags().IntVar(&lowStock, "low-stock", 0, "Low stock quantity")
	if err := cmd.MarkFlagRequired("low-stock"); err != nil {
		log.Fatal(err)
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryThresholdDeleteCmd() *cobra.Command {
	var thresholdID int

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an inventory threshold",
		Run: func(cmd *cobra.Command, args []string) {
			if thresholdID < 1 {
				log.Fatal("provide --id")
			}
			_, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.MessageResponse, error) {
				result, err := endpoints.DeleteAdminInventoryThreshold(ctx, apicontract.DeleteAdminInventoryThresholdRequestObject{Id: thresholdID})
				if err != nil {
					return apicontract.MessageResponse{}, err
				}
				return apicontract.MessageResponse(result.(apicontract.DeleteAdminInventoryThreshold200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Inventory threshold %d deleted\n", thresholdID)
		},
	}

	cmd.Flags().IntVar(&thresholdID, "id", 0, "Inventory threshold ID")
	return cmd
}

func newInventoryAdjustmentCmd() *cobra.Command {
	var variantID int
	var delta int
	var reason string
	var notes string
	var approvedByID int
	var approvedByType string
	var format string

	cmd := &cobra.Command{
		Use:   "adjust",
		Short: "Create an inventory adjustment",
		Run: func(cmd *cobra.Command, args []string) {
			if variantID < 1 {
				log.Fatal("provide --variant-id")
			}
			if delta == 0 {
				log.Fatal("--delta cannot be zero")
			}
			normalizedReason := strings.ToUpper(strings.TrimSpace(reason))
			if normalizedReason == "" {
				log.Fatal("provide --reason")
			}
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			body := apicontract.InventoryAdjustmentRequest{
				ProductVariantId: variantID,
				QuantityDelta:    delta,
				ReasonCode:       apicontract.InventoryAdjustmentReason(normalizedReason),
			}
			if strings.TrimSpace(notes) != "" {
				body.Notes = &notes
			}
			if cmd.Flags().Changed("approved-by-id") {
				if approvedByID < 1 {
					log.Fatal("--approved-by-id must be positive")
				}
				body.ApprovedById = &approvedByID
			}
			if strings.TrimSpace(approvedByType) != "" {
				body.ApprovedByType = &approvedByType
			}

			response, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryAdjustmentResponse, error) {
				result, err := endpoints.CreateAdminInventoryAdjustment(ctx, apicontract.CreateAdminInventoryAdjustmentRequestObject{Body: &body})
				if err != nil {
					return apicontract.InventoryAdjustmentResponse{}, err
				}
				return apicontract.InventoryAdjustmentResponse(result.(apicontract.CreateAdminInventoryAdjustment201JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(response)
				return
			}
			fmt.Printf("Inventory adjustment %d created\n", response.Adjustment.Id)
			fmt.Printf("Variant: %d\n", response.Adjustment.ProductVariantId)
			fmt.Printf("Delta: %+d\n", response.Adjustment.QuantityDelta)
			fmt.Printf("Reason: %s\n", response.Adjustment.ReasonCode)
			fmt.Printf("Available: %d (on hand %d, reserved %d)\n", response.Availability.Available, response.Availability.OnHand, response.Availability.Reserved)
		},
	}

	cmd.Flags().IntVar(&variantID, "variant-id", 0, "Product variant ID")
	cmd.Flags().IntVar(&delta, "delta", 0, "Quantity delta; positive adds stock and negative removes stock")
	cmd.Flags().StringVar(&reason, "reason", "", "Adjustment reason (CYCLE_COUNT_GAIN, CYCLE_COUNT_LOSS, DAMAGE, SHRINKAGE, RETURN_RESTOCK, CORRECTION)")
	cmd.Flags().StringVar(&notes, "notes", "", "Adjustment notes")
	cmd.Flags().IntVar(&approvedByID, "approved-by-id", 0, "Approver user ID")
	cmd.Flags().StringVar(&approvedByType, "approved-by-type", "", "Approver type")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryReconcileCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Run inventory reconciliation diagnostics",
		Run: func(cmd *cobra.Command, args []string) {
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			report, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryReconciliationReport, error) {
				result, err := endpoints.RunAdminInventoryReconciliation(ctx, apicontract.RunAdminInventoryReconciliationRequestObject{})
				if err != nil {
					return apicontract.InventoryReconciliationReport{}, err
				}
				return apicontract.InventoryReconciliationReport(result.(apicontract.RunAdminInventoryReconciliation200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(report)
				return
			}
			printInventoryReconciliation(report)
		},
	}

	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newInventoryTimelineCmd() *cobra.Command {
	var variantID int
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Show a product variant inventory timeline",
		Run: func(cmd *cobra.Command, args []string) {
			if variantID < 1 {
				log.Fatal("provide --variant-id")
			}
			out, err := normalizeOutputFormat(format)
			if err != nil {
				log.Fatal(err)
			}

			timeline, err := withCatalogEndpoints(cmd.Context(), func(ctx context.Context, endpoints *httpapi.CatalogEndpoints) (apicontract.InventoryTimeline, error) {
				result, err := endpoints.GetAdminInventoryTimeline(ctx, apicontract.GetAdminInventoryTimelineRequestObject{ProductVariantId: variantID, Params: apicontract.GetAdminInventoryTimelineParams{Limit: &limit}})
				if err != nil {
					return apicontract.InventoryTimeline{}, err
				}
				return apicontract.InventoryTimeline(result.(apicontract.GetAdminInventoryTimeline200JSONResponse)), nil
			})
			if err != nil {
				log.Fatal(err)
			}
			if out == outputFormatJSON {
				printJSON(timeline)
				return
			}
			printInventoryTimeline(timeline)
		},
	}

	cmd.Flags().IntVar(&variantID, "variant-id", 0, "Product variant ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum records per timeline section")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func inventoryListPath(basePath string, statuses []string, limit int, defaultLimit int) string {
	values := url.Values{}
	for _, status := range statuses {
		for _, part := range strings.Split(status, ",") {
			trimmed := strings.ToUpper(strings.TrimSpace(part))
			if trimmed != "" {
				values.Add("status", trimmed)
			}
		}
	}
	if limit > 0 && limit != defaultLimit {
		values.Set("limit", strconv.Itoa(limit))
	}
	if encoded := values.Encode(); encoded != "" {
		return basePath + "?" + encoded
	}
	return basePath
}

func inventoryAlertActionPastTense(action string) string {
	switch action {
	case "ack":
		return "acked"
	case "resolve":
		return "resolved"
	default:
		return action
	}
}

func printInventoryReservations(items []apicontract.InventoryReservation) {
	if len(items) == 0 {
		fmt.Println("No inventory reservations found")
		return
	}
	fmt.Printf("%-6s %-9s %-12s %-5s %-20s %-14s\n", "ID", "Variant", "Status", "Qty", "Expires", "Owner")
	for _, item := range items {
		fmt.Printf("%-6d %-9d %-12s %-5d %-20s %-14s\n",
			item.Id,
			item.ProductVariantId,
			item.Status,
			item.Quantity,
			item.ExpiresAt.Format("2006-01-02 15:04"),
			inventoryOwnerLabel(item.OwnerType, item.OwnerId),
		)
	}
}

func printInventoryAlerts(items []apicontract.InventoryAlert) {
	if len(items) == 0 {
		fmt.Println("No inventory alerts found")
		return
	}
	fmt.Printf("%-6s %-9s %-14s %-12s %-10s %-10s\n", "ID", "Variant", "Type", "Status", "Available", "Threshold")
	for _, item := range items {
		fmt.Printf("%-6d %-9d %-14s %-12s %-10d %-10d\n",
			item.Id,
			item.ProductVariantId,
			item.AlertType,
			item.Status,
			item.Available,
			item.Threshold,
		)
	}
}

func printInventoryThresholds(items []apicontract.InventoryThreshold) {
	if len(items) == 0 {
		fmt.Println("No inventory thresholds found")
		return
	}
	fmt.Printf("%-6s %-12s %-10s\n", "ID", "Variant", "Low stock")
	for _, item := range items {
		fmt.Printf("%-6d %-12s %-10d\n", item.Id, thresholdVariantLabel(item.ProductVariantId), item.LowStockQuantity)
	}
}

func printInventoryReconciliation(report apicontract.InventoryReconciliationReport) {
	fmt.Printf("Checked at: %s\n", report.CheckedAt.Format("2006-01-02 15:04:05 MST"))
	if len(report.Issues) == 0 {
		fmt.Println("No inventory reconciliation issues found")
		return
	}
	fmt.Printf("%-22s %-9s %-10s %-10s %-16s\n", "Issue", "Variant", "Expected", "Actual", "Entity")
	for _, issue := range report.Issues {
		fmt.Printf("%-22s %-9d %-10d %-10d %-16s\n",
			issue.IssueType,
			issue.ProductVariantId,
			issue.Expected,
			issue.Actual,
			inventoryEntityLabel(issue.EntityType, issue.EntityId),
		)
	}
}

func printInventoryTimeline(timeline apicontract.InventoryTimeline) {
	fmt.Printf("Variant: %d\n", timeline.ProductVariantId)
	fmt.Printf("Adjustments: %d\n", len(timeline.Adjustments))
	for _, item := range timeline.Adjustments {
		fmt.Printf("  %s adjustment #%d %+d %s\n", item.CreatedAt.Format("2006-01-02 15:04"), item.Id, item.QuantityDelta, item.ReasonCode)
	}
	fmt.Printf("Movements: %d\n", len(timeline.Movements))
	for _, item := range timeline.Movements {
		fmt.Printf("  %s movement #%d %+d %s %s\n", item.CreatedAt.Format("2006-01-02 15:04"), item.Id, item.QuantityDelta, item.MovementType, item.ReasonCode)
	}
	fmt.Printf("Reservations: %d\n", len(timeline.Reservations))
	for _, item := range timeline.Reservations {
		fmt.Printf("  %s reservation #%d %s qty %d expires %s\n", item.CreatedAt.Format("2006-01-02 15:04"), item.Id, item.Status, item.Quantity, item.ExpiresAt.Format("2006-01-02 15:04"))
	}
}

func thresholdVariantLabel(variantID *int) string {
	if variantID == nil {
		return "default"
	}
	return strconv.Itoa(*variantID)
}

func inventoryOwnerLabel(ownerType string, ownerID *int) string {
	if ownerID == nil {
		return strings.TrimSpace(ownerType)
	}
	if strings.TrimSpace(ownerType) == "" {
		return strconv.Itoa(*ownerID)
	}
	return fmt.Sprintf("%s:%d", ownerType, *ownerID)
}

func inventoryEntityLabel(entityType string, entityID *int) string {
	if entityID == nil {
		return strings.TrimSpace(entityType)
	}
	if strings.TrimSpace(entityType) == "" {
		return strconv.Itoa(*entityID)
	}
	return fmt.Sprintf("%s:%d", entityType, *entityID)
}
