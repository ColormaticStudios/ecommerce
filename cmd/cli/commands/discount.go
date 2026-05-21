package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ecommerce/handlers"
	"ecommerce/internal/apicontract"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func NewDiscountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discount",
		Short: "Discount and promotion administration commands",
	}

	cmd.AddCommand(newListDiscountCampaignsCmd())
	cmd.AddCommand(newCreateProductDiscountCmd())
	cmd.AddCommand(newUpdateProductDiscountCmd())
	cmd.AddCommand(newDisableDiscountCampaignCmd())
	cmd.AddCommand(newArchiveDiscountCampaignCmd())
	cmd.AddCommand(newScheduleDiscountCampaignCmd())
	cmd.AddCommand(newRunDiscountLifecycleCmd())
	cmd.AddCommand(newListDiscountHistoryCmd())
	cmd.AddCommand(newListDiscountAuditCmd())
	cmd.AddCommand(newGetDiscountMetricsCmd())
	cmd.AddCommand(newRunDiscountReconciliationCmd())
	cmd.AddCommand(newCreatePromotionCmd())
	cmd.AddCommand(newPreviewPromotionCmd())
	cmd.AddCommand(newListPromotionTemplatesCmd())
	cmd.AddCommand(newCreatePromotionTemplateCmd())
	cmd.AddCommand(newInstantiatePromotionTemplateCmd())

	return cmd
}

type productDiscountFlags struct {
	name                string
	productIDs          []int
	discountMode        string
	discountValue       float64
	startsAt            string
	endsAt              string
	priority            int
	isExclusive         bool
	status              string
	metadataFile        string
	couponCode          string
	channels            []string
	customerSegment     string
	globalUsageCap      int
	perCustomerUsageCap int
}

func newListDiscountCampaignsCmd() *cobra.Command {
	var format string
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discount campaigns",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/discounts/campaigns"
			if trimmed := strings.TrimSpace(status); trimmed != "" {
				path += "?status=" + url.QueryEscape(trimmed)
			}
			resp, err := invokeWithDB[apicontract.DiscountCampaignListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminDiscountCampaigns(db)
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
			printDiscountCampaigns(resp.Campaigns)
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status: active, scheduled, disabled, or archived")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreateProductDiscountCmd() *cobra.Command {
	var input productDiscountFlags
	var format string
	cmd := &cobra.Command{
		Use:   "create-product",
		Short: "Create a product discount campaign",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := input.toContract(cmd)
			if err != nil {
				return err
			}
			campaign, err := invokeDiscountCampaignMutation(http.MethodPost, "/api/v1/admin/discounts/campaigns", nil, payload, handlers.CreateAdminDiscountCampaign)
			if err != nil {
				return err
			}
			return printDiscountCampaignMutation(campaign, format, "created")
		},
	}
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	markRequired(cmd, "name", "product-id", "discount-mode", "discount-value", "starts-at")
	return cmd
}

func newUpdateProductDiscountCmd() *cobra.Command {
	var id uint
	var input productDiscountFlags
	var format string
	cmd := &cobra.Command{
		Use:   "update-product",
		Short: "Update a product discount campaign",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := input.toContract(cmd)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/api/v1/admin/discounts/campaigns/%d", id)
			campaign, err := invokeDiscountCampaignMutation(http.MethodPatch, path, map[string]string{"id": fmt.Sprintf("%d", id)}, payload, handlers.UpdateAdminDiscountCampaign)
			if err != nil {
				return err
			}
			return printDiscountCampaignMutation(campaign, format, "updated")
		},
	}
	cmd.Flags().UintVar(&id, "id", 0, "Discount campaign ID")
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	markRequired(cmd, "id", "name", "product-id", "discount-mode", "discount-value", "starts-at")
	return cmd
}

func newDisableDiscountCampaignCmd() *cobra.Command {
	return newSimpleDiscountCampaignActionCmd("disable", "Disable a discount campaign", "disabled", handlers.DisableAdminDiscountCampaign)
}

func newArchiveDiscountCampaignCmd() *cobra.Command {
	return newSimpleDiscountCampaignActionCmd("archive", "Archive a discount campaign", "archived", handlers.ArchiveAdminDiscountCampaign)
}

func newScheduleDiscountCampaignCmd() *cobra.Command {
	var id uint
	var scheduleType string
	var recurrence string
	var windowStart string
	var windowEnd string
	var untilAt string
	var timezoneName string
	var format string

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Create or update a discount campaign schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			start, err := parseCLITime(windowStart, "window-start")
			if err != nil {
				return err
			}
			end, err := parseCLITime(windowEnd, "window-end")
			if err != nil {
				return err
			}
			var until *time.Time
			if strings.TrimSpace(untilAt) != "" {
				parsed, err := parseCLITime(untilAt, "until-at")
				if err != nil {
					return err
				}
				until = &parsed
			}
			var recurrencePtr *apicontract.DiscountScheduleInputRecurrence
			if strings.TrimSpace(recurrence) != "" {
				value := apicontract.DiscountScheduleInputRecurrence(strings.TrimSpace(recurrence))
				recurrencePtr = &value
			}
			var timezonePtr *string
			if strings.TrimSpace(timezoneName) != "" {
				value := strings.TrimSpace(timezoneName)
				timezonePtr = &value
			}
			payload := apicontract.DiscountScheduleInput{
				ScheduleType: apicontract.DiscountScheduleInputScheduleType(strings.TrimSpace(scheduleType)),
				Recurrence:   recurrencePtr,
				WindowStart:  start,
				WindowEnd:    end,
				UntilAt:      until,
				Timezone:     timezonePtr,
			}
			path := fmt.Sprintf("/api/v1/admin/discounts/campaigns/%d/schedule", id)
			schedule, err := invokeWithDB[apicontract.DiscountSchedule](localHandlerRequest{
				Method:     http.MethodPost,
				Path:       path,
				PathParams: map[string]string{"id": fmt.Sprintf("%d", id)},
				Body:       payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ScheduleAdminDiscountCampaign(db)
			})
			if err != nil {
				return err
			}
			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(schedule)
				return nil
			}
			fmt.Printf("✓ Schedule saved for campaign %d (next run: %s)\n", schedule.CampaignId, formatOptionalTime(schedule.NextRunAt))
			return nil
		},
	}
	cmd.Flags().UintVar(&id, "id", 0, "Discount campaign ID")
	cmd.Flags().StringVar(&scheduleType, "type", "one_time", "Schedule type: one_time or recurring")
	cmd.Flags().StringVar(&recurrence, "recurrence", "", "Recurring cadence: daily, weekly, or monthly")
	cmd.Flags().StringVar(&windowStart, "window-start", "", "Window start time (RFC3339)")
	cmd.Flags().StringVar(&windowEnd, "window-end", "", "Window end time (RFC3339)")
	cmd.Flags().StringVar(&untilAt, "until-at", "", "Optional recurrence end time (RFC3339)")
	cmd.Flags().StringVar(&timezoneName, "timezone", "", "Schedule timezone")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	markRequired(cmd, "id", "window-start", "window-end")
	return cmd
}

func newRunDiscountLifecycleCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "run-lifecycle",
		Short: "Run discount lifecycle processing",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.DiscountLifecycleRunResponse](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/discounts/lifecycle/run",
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.RunAdminDiscountLifecycle(db)
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
			fmt.Printf("Activated: %d\nDeactivated: %d\nArchived: %d\n", resp.Activated, resp.Deactivated, resp.Archived)
			return nil
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newListDiscountHistoryCmd() *cobra.Command {
	var campaignID uint
	var format string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List discount state history",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := discountCampaignScopedPath("/api/v1/admin/discounts/history", campaignID)
			resp, err := invokeWithDB[apicontract.DiscountStateHistoryListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminDiscountHistory(db)
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
			if len(resp.History) == 0 {
				fmt.Println("No discount history found")
				return nil
			}
			fmt.Printf("%-5s %-8s %-12s %-12s %-16s %-20s\n", "ID", "Campaign", "From", "To", "Source", "Changed")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, entry := range resp.History {
				fmt.Printf("%-5d %-8d %-12s %-12s %-16s %-20s\n", entry.Id, entry.CampaignId, entry.FromStatus, entry.ToStatus, entry.Source, entry.ChangedAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	cmd.Flags().UintVar(&campaignID, "campaign-id", 0, "Filter by campaign ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newListDiscountAuditCmd() *cobra.Command {
	var campaignID uint
	var format string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List discount audit entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := discountCampaignScopedPath("/api/v1/admin/discounts/audit", campaignID)
			resp, err := invokeWithDB[apicontract.DiscountCampaignAuditListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminDiscountAudit(db)
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
			if len(resp.Audit) == 0 {
				fmt.Println("No discount audit entries found")
				return nil
			}
			fmt.Printf("%-5s %-8s %-14s %-16s %-20s %s\n", "ID", "Campaign", "Event", "Source", "Changed", "Summary")
			fmt.Println("------------------------------------------------------------------------------------------------")
			for _, entry := range resp.Audit {
				fmt.Printf("%-5d %-8d %-14s %-16s %-20s %s\n", entry.Id, entry.CampaignId, entry.EventType, entry.Source, entry.ChangedAt.Format(time.RFC3339), entry.Summary)
			}
			return nil
		},
	}
	cmd.Flags().UintVar(&campaignID, "campaign-id", 0, "Filter by campaign ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newGetDiscountMetricsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show discount evaluation metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeJSON[apicontract.DiscountEvaluationMetrics](handlers.GetAdminDiscountMetrics(), localHandlerRequest{
				Method: http.MethodGet,
				Path:   "/api/v1/admin/discounts/metrics",
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
			fmt.Printf("Total evaluations: %d\nMatched evaluations: %d\nFailed evaluations: %d\nLast latency: %dms\nLast error: %s\n",
				resp.TotalEvaluations, resp.MatchedEvaluations, resp.FailedEvaluations, resp.LastLatencyMs, resp.LastError)
			return nil
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newRunDiscountReconciliationCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Run discount schedule reconciliation",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := invokeWithDB[apicontract.DiscountReconciliationReport](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/discounts/reconciliation/run",
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.RunAdminDiscountReconciliation(db)
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
			if len(resp.Issues) == 0 {
				fmt.Printf("No reconciliation issues found at %s\n", resp.CheckedAt.Format(time.RFC3339))
				return nil
			}
			fmt.Printf("%-8s %-8s %-12s %-12s %s\n", "Campaign", "Schedule", "Expected", "Actual", "Message")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, issue := range resp.Issues {
				fmt.Printf("%-8d %-8d %-12s %-12s %s\n", issue.CampaignId, issue.ScheduleId, issue.ExpectedStatus, issue.ActualStatus, issue.Message)
			}
			return nil
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreatePromotionCmd() *cobra.Command {
	var inputFile string
	var format string
	cmd := &cobra.Command{
		Use:   "create-promotion",
		Short: "Create a promotion campaign from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload apicontract.PromotionInput
			if err := loadJSONFile(inputFile, &payload); err != nil {
				return err
			}
			campaign, err := invokeDiscountCampaignMutation(http.MethodPost, "/api/v1/admin/discounts/promotions", nil, payload, handlers.CreateAdminPromotionCampaign)
			if err != nil {
				return err
			}
			return printDiscountCampaignMutation(campaign, format, "created")
		},
	}
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to PromotionInput JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("input")
	return cmd
}

func newPreviewPromotionCmd() *cobra.Command {
	var inputFile string
	var format string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview promotion evaluation from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload apicontract.PromotionEvaluationRequest
			if err := loadJSONFile(inputFile, &payload); err != nil {
				return err
			}
			resp, err := invokeWithDB[apicontract.PromotionEvaluationResponse](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/discounts/promotions/preview",
				Body:   payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.PreviewAdminPromotion(db)
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
			fmt.Printf("Subtotal: $%.2f\nDiscount: $%.2f\nFinal subtotal: $%.2f\nLines: %d\n", resp.Subtotal, resp.DiscountTotal, resp.FinalSubtotal, len(resp.Lines))
			return nil
		},
	}
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to PromotionEvaluationRequest JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("input")
	return cmd
}

func newListPromotionTemplatesCmd() *cobra.Command {
	var includeInactive bool
	var format string
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List promotion templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/discounts/templates"
			if includeInactive {
				path += "?active=false"
			}
			resp, err := invokeWithDB[apicontract.PromotionTemplateListResponse](localHandlerRequest{
				Method: http.MethodGet,
				Path:   path,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.ListAdminPromotionTemplates(db)
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
			if len(resp.Templates) == 0 {
				fmt.Println("No promotion templates found")
				return nil
			}
			fmt.Printf("%-5s %-28s %-8s %s\n", "ID", "Name", "Active", "Description")
			fmt.Println("----------------------------------------------------------------")
			for _, template := range resp.Templates {
				fmt.Printf("%-5d %-28s %-8t %s\n", template.Id, template.Name, template.IsActive, template.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&includeInactive, "include-inactive", false, "Include inactive templates")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCreatePromotionTemplateCmd() *cobra.Command {
	var inputFile string
	var format string
	cmd := &cobra.Command{
		Use:   "create-template",
		Short: "Create a promotion template from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload apicontract.PromotionTemplateInput
			if err := loadJSONFile(inputFile, &payload); err != nil {
				return err
			}
			template, err := invokeWithDB[apicontract.PromotionTemplate](localHandlerRequest{
				Method: http.MethodPost,
				Path:   "/api/v1/admin/discounts/templates",
				Body:   payload,
			}, func(db *gorm.DB) gin.HandlerFunc {
				return handlers.CreateAdminPromotionTemplate(db)
			})
			if err != nil {
				return err
			}
			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(template)
				return nil
			}
			fmt.Printf("✓ Promotion template created: %s (ID: %d)\n", template.Name, template.Id)
			return nil
		},
	}
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to PromotionTemplateInput JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("input")
	return cmd
}

func newInstantiatePromotionTemplateCmd() *cobra.Command {
	var id uint
	var inputFile string
	var format string
	cmd := &cobra.Command{
		Use:   "instantiate-template",
		Short: "Instantiate a promotion template from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload apicontract.PromotionTemplateInstantiateInput
			if err := loadJSONFile(inputFile, &payload); err != nil {
				return err
			}
			path := fmt.Sprintf("/api/v1/admin/discounts/templates/%d/instantiate", id)
			campaign, err := invokeDiscountCampaignMutation(http.MethodPost, path, map[string]string{"id": fmt.Sprintf("%d", id)}, payload, handlers.InstantiateAdminPromotionTemplate)
			if err != nil {
				return err
			}
			return printDiscountCampaignMutation(campaign, format, "created")
		},
	}
	cmd.Flags().UintVar(&id, "id", 0, "Promotion template ID")
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to PromotionTemplateInstantiateInput JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	markRequired(cmd, "id", "input")
	return cmd
}

func (f *productDiscountFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Campaign name")
	cmd.Flags().IntSliceVar(&f.productIDs, "product-id", nil, "Product ID to discount (repeatable or comma-separated)")
	cmd.Flags().StringVar(&f.discountMode, "discount-mode", "", "Discount mode: percent or fixed")
	cmd.Flags().Float64Var(&f.discountValue, "discount-value", 0, "Discount value")
	cmd.Flags().StringVar(&f.startsAt, "starts-at", "", "Start time (RFC3339)")
	cmd.Flags().StringVar(&f.endsAt, "ends-at", "", "Optional end time (RFC3339)")
	cmd.Flags().IntVar(&f.priority, "priority", 0, "Campaign priority")
	cmd.Flags().BoolVar(&f.isExclusive, "exclusive", false, "Prevent lower-priority discounts from stacking")
	cmd.Flags().StringVar(&f.status, "status", "active", "Campaign status: active or disabled")
	cmd.Flags().StringVar(&f.metadataFile, "metadata", "", "Path to metadata JSON object")
	cmd.Flags().StringVar(&f.couponCode, "coupon-code", "", "Optional coupon code")
	cmd.Flags().StringSliceVar(&f.channels, "channel", nil, "Eligible channel: web, app, or admin (repeatable or comma-separated)")
	cmd.Flags().StringVar(&f.customerSegment, "customer-segment", "", "Optional customer segment")
	cmd.Flags().IntVar(&f.globalUsageCap, "global-usage-cap", 0, "Optional global usage cap")
	cmd.Flags().IntVar(&f.perCustomerUsageCap, "per-customer-usage-cap", 0, "Optional per-customer usage cap")
}

func (f productDiscountFlags) toContract(cmd *cobra.Command) (apicontract.ProductDiscountInput, error) {
	startsAt, err := parseCLITime(f.startsAt, "starts-at")
	if err != nil {
		return apicontract.ProductDiscountInput{}, err
	}
	var endsAt *time.Time
	if strings.TrimSpace(f.endsAt) != "" {
		parsed, err := parseCLITime(f.endsAt, "ends-at")
		if err != nil {
			return apicontract.ProductDiscountInput{}, err
		}
		endsAt = &parsed
	}
	productIDs := make([]int, 0, len(f.productIDs))
	for _, id := range f.productIDs {
		if id <= 0 {
			return apicontract.ProductDiscountInput{}, fmt.Errorf("product IDs must be positive")
		}
		productIDs = append(productIDs, id)
	}
	var metadata *map[string]any
	if strings.TrimSpace(f.metadataFile) != "" {
		value := map[string]any{}
		if err := loadJSONFile(f.metadataFile, &value); err != nil {
			return apicontract.ProductDiscountInput{}, err
		}
		metadata = &value
	}
	var couponCode *string
	if strings.TrimSpace(f.couponCode) != "" {
		value := strings.TrimSpace(f.couponCode)
		couponCode = &value
	}
	var customerSegment *string
	if strings.TrimSpace(f.customerSegment) != "" {
		value := strings.TrimSpace(f.customerSegment)
		customerSegment = &value
	}
	var channels *[]apicontract.ProductDiscountInputChannels
	if len(f.channels) > 0 {
		values := make([]apicontract.ProductDiscountInputChannels, 0, len(f.channels))
		for _, channel := range expandStringList(f.channels) {
			values = append(values, apicontract.ProductDiscountInputChannels(channel))
		}
		channels = &values
	}
	var globalUsageCap *int
	if f.globalUsageCap > 0 {
		globalUsageCap = &f.globalUsageCap
	}
	var perCustomerUsageCap *int
	if f.perCustomerUsageCap > 0 {
		perCustomerUsageCap = &f.perCustomerUsageCap
	}
	status := apicontract.ProductDiscountInputStatus(strings.TrimSpace(f.status))
	return apicontract.ProductDiscountInput{
		Name:                strings.TrimSpace(f.name),
		ProductIds:          productIDs,
		DiscountMode:        apicontract.ProductDiscountInputDiscountMode(strings.TrimSpace(f.discountMode)),
		DiscountValue:       f.discountValue,
		StartsAt:            startsAt,
		EndsAt:              endsAt,
		Priority:            &f.priority,
		IsExclusive:         &f.isExclusive,
		Status:              &status,
		Metadata:            metadata,
		CouponCode:          couponCode,
		Channels:            channels,
		CustomerSegment:     customerSegment,
		GlobalUsageCap:      globalUsageCap,
		PerCustomerUsageCap: perCustomerUsageCap,
	}, nil
}

func newSimpleDiscountCampaignActionCmd(name string, short string, pastTense string, handlerFactory func(*gorm.DB) gin.HandlerFunc) *cobra.Command {
	var id uint
	var format string
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/admin/discounts/campaigns/%d/%s", id, name)
			campaign, err := invokeDiscountCampaignMutation(http.MethodPost, path, map[string]string{"id": fmt.Sprintf("%d", id)}, nil, handlerFactory)
			if err != nil {
				return err
			}
			return printDiscountCampaignMutation(campaign, format, pastTense)
		},
	}
	cmd.Flags().UintVar(&id, "id", 0, "Discount campaign ID")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("id")
	return cmd
}

func invokeDiscountCampaignMutation(method string, path string, pathParams map[string]string, body any, handlerFactory func(*gorm.DB) gin.HandlerFunc) (apicontract.DiscountCampaign, error) {
	return invokeWithDB[apicontract.DiscountCampaign](localHandlerRequest{
		Method:     method,
		Path:       path,
		PathParams: pathParams,
		Body:       body,
	}, handlerFactory)
}

func printDiscountCampaignMutation(campaign apicontract.DiscountCampaign, format string, action string) error {
	selectedFormat, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}
	if selectedFormat == outputFormatJSON {
		printJSON(campaign)
		return nil
	}
	fmt.Printf("✓ Discount campaign %s: %s (ID: %d, status: %s)\n", action, campaign.Name, campaign.Id, campaign.Status)
	return nil
}

func printDiscountCampaigns(campaigns []apicontract.DiscountCampaign) {
	if len(campaigns) == 0 {
		fmt.Println("No discount campaigns found")
		return
	}
	fmt.Printf("%-5s %-30s %-16s %-12s %-10s %-8s %-20s\n", "ID", "Name", "Type", "Status", "Mode", "Value", "Starts")
	fmt.Println("------------------------------------------------------------------------------------------------")
	for _, campaign := range campaigns {
		fmt.Printf("%-5d %-30s %-16s %-12s %-10s %-8.2f %-20s\n",
			campaign.Id,
			campaign.Name,
			campaign.Type,
			campaign.Status,
			campaign.DiscountMode,
			campaign.DiscountValue,
			campaign.StartsAt.Format(time.RFC3339),
		)
	}
}

func parseCLITime(raw string, flagName string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse --%s as RFC3339: %w", flagName, err)
	}
	return parsed, nil
}

func expandStringList(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}

func discountCampaignScopedPath(base string, campaignID uint) string {
	if campaignID == 0 {
		return base
	}
	return base + "?campaign_id=" + url.QueryEscape(fmt.Sprintf("%d", campaignID))
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "not scheduled"
	}
	return value.Format(time.RFC3339)
}

func markRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		_ = cmd.MarkFlagRequired(name)
	}
}
