package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/services/cms"
	"ecommerce/models"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type cmsDraftEnvelope struct {
	Path          string          `json:"path,omitempty"`
	Slug          string          `json:"slug,omitempty"`
	Title         string          `json:"title,omitempty"`
	TemplateKey   string          `json:"template_key,omitempty"`
	Visibility    string          `json:"visibility,omitempty"`
	IsHomepage    bool            `json:"is_homepage,omitempty"`
	Key           string          `json:"key,omitempty"`
	Location      string          `json:"location,omitempty"`
	Region        string          `json:"region,omitempty"`
	Items         json.RawMessage `json:"items,omitempty"`
	Payload       cms.PagePayload `json:"payload,omitempty"`
	ChangeSummary string          `json:"change_summary,omitempty"`
}

type cmsSEOEnvelope struct {
	Title               string           `json:"title"`
	Description         string           `json:"description"`
	CanonicalURL        string           `json:"canonical_url"`
	Robots              string           `json:"robots"`
	OGTitle             string           `json:"og_title"`
	OGDescription       string           `json:"og_description"`
	OGImageMediaID      *string          `json:"og_image_media_id"`
	TwitterCard         string           `json:"twitter_card"`
	TwitterTitle        string           `json:"twitter_title"`
	TwitterDescription  string           `json:"twitter_description"`
	TwitterImageMediaID *string          `json:"twitter_image_media_id"`
	JSONLD              []map[string]any `json:"json_ld"`
}

type cmsDeliveryEnvelope struct {
	Schedule *struct {
		PublishAt   string  `json:"publish_at"`
		UnpublishAt *string `json:"unpublish_at"`
		Timezone    string  `json:"timezone"`
	} `json:"schedule"`
	TargetingRules []struct {
		Markets       []string `json:"markets"`
		DeviceClasses []string `json:"device_classes"`
		AuthStates    []string `json:"auth_states"`
		Referrers     []string `json:"referrers"`
		UTMSources    []string `json:"utm_sources"`
		SegmentKeys   []string `json:"segment_keys"`
		Priority      int      `json:"priority"`
		IsEnabled     bool     `json:"is_enabled"`
	} `json:"targeting_rules"`
	Experiment *struct {
		Name      string  `json:"name"`
		Status    string  `json:"status"`
		StickyKey string  `json:"sticky_key"`
		StartsAt  string  `json:"starts_at"`
		EndsAt    *string `json:"ends_at"`
		Variants  []struct {
			Name       string `json:"name"`
			VersionID  uint   `json:"version_id"`
			Allocation int    `json:"allocation"`
		} `json:"variants"`
	} `json:"experiment"`
}

type cmsLocaleEnvelope struct {
	Locales []struct {
		Code           string `json:"code"`
		Name           string `json:"name"`
		Enabled        bool   `json:"enabled"`
		IsDefault      bool   `json:"is_default"`
		FallbackLocale string `json:"fallback_locale"`
	} `json:"locales"`
}

type cmsVariantEnvelope struct {
	Locale        string          `json:"locale"`
	Market        string          `json:"market"`
	Path          string          `json:"path"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Payload       cms.PagePayload `json:"payload"`
	ChangeSummary string          `json:"change_summary"`
}

type cmsGovernanceEnvelope struct {
	ApprovalRequired       bool   `json:"approval_required"`
	InvalidationWebhookURL string `json:"invalidation_webhook_url"`
	Roles                  []struct {
		Subject string `json:"subject"`
		Role    string `json:"role"`
	} `json:"roles"`
}

type cmsNavigationItemEnvelope struct {
	ID        uint   `json:"id,omitempty"`
	ParentID  *uint  `json:"parent_id,omitempty"`
	Label     string `json:"label"`
	ItemType  string `json:"item_type"`
	TargetRef string `json:"target_ref,omitempty"`
	URL       string `json:"url,omitempty"`
	SortOrder int    `json:"sort_order"`
	IsEnabled bool   `json:"is_enabled"`
}

type cmsExportVersion struct {
	ID            uint            `json:"id"`
	EntryID       uint            `json:"entry_id"`
	VersionNumber uint            `json:"version_number"`
	SchemaVersion uint            `json:"schema_version"`
	Payload       cms.PagePayload `json:"payload"`
	CreatedBy     *uint           `json:"created_by"`
	ChangeSummary *string         `json:"change_summary"`
	CreatedAt     time.Time       `json:"created_at"`
}

type cmsExportPage struct {
	Page              models.CMSPage         `json:"page"`
	Entry             models.CMSEntry        `json:"entry"`
	CurrentVersion    *cmsExportVersion      `json:"current_version"`
	PublishedVersion  *cmsExportVersion      `json:"published_version"`
	LatestPublication *models.CMSPublication `json:"latest_publication"`
}

type cmsExportNavigation struct {
	Menu              models.CMSNavigationMenu   `json:"menu"`
	Entry             models.CMSEntry            `json:"entry"`
	Items             []models.CMSNavigationItem `json:"items"`
	CurrentVersion    *cmsExportVersion          `json:"current_version"`
	PublishedVersion  *cmsExportVersion          `json:"published_version"`
	LatestPublication *models.CMSPublication     `json:"latest_publication"`
}

type cmsExportGlobalRegion struct {
	Region            models.CMSGlobalRegion `json:"region"`
	Entry             models.CMSEntry        `json:"entry"`
	CurrentVersion    *cmsExportVersion      `json:"current_version"`
	PublishedVersion  *cmsExportVersion      `json:"published_version"`
	LatestPublication *models.CMSPublication `json:"latest_publication"`
}

type cmsExportVariant struct {
	ID          uint            `json:"id"`
	PageID      uint            `json:"page_id"`
	EntryID     uint            `json:"entry_id"`
	Locale      string          `json:"locale"`
	Market      string          `json:"market"`
	Path        string          `json:"path"`
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Payload     cms.PagePayload `json:"payload"`
	Status      string          `json:"status"`
	Revision    uint            `json:"revision"`
	SubmittedBy *string         `json:"submitted_by"`
	ApprovedBy  *string         `json:"approved_by"`
	PublishedAt *time.Time      `json:"published_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func NewCMSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cms",
		Short: "CMS content controls",
	}

	cmd.AddCommand(newCMSExportCmd())
	cmd.AddCommand(newCMSRestoreCmd())
	cmd.AddCommand(newCMSPageCmd())
	cmd.AddCommand(newCMSNavigationCmd())
	cmd.AddCommand(newCMSGlobalCmd())
	cmd.AddCommand(newCMSRedirectCmd())
	cmd.AddCommand(newCMSLocaleCmd())
	cmd.AddCommand(newCMSGovernanceCmd())
	cmd.AddCommand(newCMSAuditCmd())
	cmd.AddCommand(newCMSOperationsCmd())
	cmd.AddCommand(newCMSScaffoldCmd())
	cmd.AddCommand(newCMSBootstrapCmd())

	return cmd
}

func newCMSExportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export CMS content",
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := cmsExportContent()
			if err != nil {
				return err
			}
			if outputPath != "" {
				if err := writeJSONFile(outputPath, value); err != nil {
					return err
				}
				fmt.Printf("cms_export_path=%s\n", outputPath)
				return nil
			}
			printJSON(value)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputPath, "out", "", "Write CMS export JSON to a file")
	return cmd
}

func newCMSRestoreCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore CMS content from an export",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			if err := cmsRestoreContent(payload); err != nil {
				return err
			}
			fmt.Println("✓ CMS content restored")
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to CMS export JSON")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCMSPageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "page", Short: "CMS page controls"}
	cmd.AddCommand(newCMSPageListCmd())
	cmd.AddCommand(newCMSPageGetCmd())
	cmd.AddCommand(newCMSPageSaveCmd(false))
	cmd.AddCommand(newCMSPageSaveCmd(true))
	cmd.AddCommand(newCMSPagePublishCmd())
	cmd.AddCommand(newCMSPageSEOCmd())
	cmd.AddCommand(newCMSPageDeliveryCmd())
	cmd.AddCommand(newCMSPageVariantCmd())
	return cmd
}

func newCMSNavigationCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "navigation", Short: "CMS navigation controls"}
	cmd.AddCommand(newCMSNavigationListCmd())
	cmd.AddCommand(newCMSNavigationGetCmd())
	cmd.AddCommand(newCMSNavigationSaveCmd(false))
	cmd.AddCommand(newCMSNavigationSaveCmd(true))
	cmd.AddCommand(newCMSNavigationPublishCmd())
	return cmd
}

func newCMSGlobalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "global", Short: "CMS global region controls"}
	cmd.AddCommand(newCMSGlobalListCmd())
	cmd.AddCommand(newCMSGlobalGetCmd())
	cmd.AddCommand(newCMSGlobalSaveCmd(false))
	cmd.AddCommand(newCMSGlobalSaveCmd(true))
	cmd.AddCommand(newCMSGlobalPublishCmd())
	return cmd
}

func newCMSPageListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CMS pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			records, total, err := cmsListPages()
			if err != nil {
				return err
			}
			return printCMSList(format, records, total, func(row any) string {
				record := row.(cms.PageRecord)
				return fmt.Sprintf("%d\t%s\t%s\t%s\tdraft=%t", record.Page.ID, record.Page.Title, record.Page.Path, record.Entry.Status, record.HasUnpublishedDraft)
			})
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSPageGetCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a CMS page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsGetPage(id)
			if err != nil {
				return err
			}
			if selected, err := normalizeOutputFormat(format); err != nil {
				return err
			} else if selected == outputFormatJSON {
				printJSON(record)
			} else {
				fmt.Printf("%d\t%s\t%s\t%s\tdraft=%t\n", record.Page.ID, record.Page.Title, record.Page.Path, record.Entry.Status, record.HasUnpublishedDraft)
			}
			return nil
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSPageSaveCmd(update bool) *cobra.Command {
	var filePath string
	var format string
	use := "create"
	short := "Create a CMS page draft"
	if update {
		use = "update <id>"
		short = "Update a CMS page draft"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if update {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.NoArgs(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var input cms.PageDraftInput
			if err := loadCMSPageInput(filePath, &input); err != nil {
				return err
			}
			var id uint
			var err error
			if update {
				id, err = parseCMSID(args[0])
				if err != nil {
					return err
				}
			}
			record, err := cmsSavePage(id, input)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS page draft saved: %s (ID: %d)", record.Page.Title, record.Page.ID))
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to page draft JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCMSPagePublishCmd() *cobra.Command {
	var notes, format string
	cmd := &cobra.Command{
		Use:   "publish <id>",
		Short: "Publish a CMS page draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsPublishPage(id, notes)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS page published: %s (ID: %d)", record.Page.Title, record.Page.ID))
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Publication notes")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSPageSEOCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "seo", Short: "CMS page SEO controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "get <page-id>",
		Short: "Get CMS page SEO metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsGetPageSEO(id)
			if err != nil {
				return err
			}
			printJSON(record)
			return nil
		},
	})
	var filePath string
	cmd.AddCommand(func() *cobra.Command {
		saveCmd := &cobra.Command{
			Use:   "set <page-id>",
			Short: "Update CMS page SEO metadata from JSON",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				id, err := parseCMSID(args[0])
				if err != nil {
					return err
				}
				input, err := loadCMSSEOInput(filePath)
				if err != nil {
					return err
				}
				record, err := cmsSetPageSEO(id, input)
				if err != nil {
					return err
				}
				printJSON(record)
				return nil
			},
		}
		saveCmd.Flags().StringVar(&filePath, "file", "", "Path to SEO JSON")
		saveCmd.MarkFlagRequired("file")
		return saveCmd
	}())
	return cmd
}

func newCMSPageDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "delivery", Short: "CMS page delivery controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "get <page-id>",
		Short: "Get CMS page delivery settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsGetPageDelivery(id)
			if err != nil {
				return err
			}
			printJSON(record)
			return nil
		},
	})
	var filePath string
	cmd.AddCommand(func() *cobra.Command {
		saveCmd := &cobra.Command{
			Use:   "set <page-id>",
			Short: "Update CMS page scheduling, targeting, and experiment settings from JSON",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				id, err := parseCMSID(args[0])
				if err != nil {
					return err
				}
				input, err := loadCMSDeliveryInput(filePath)
				if err != nil {
					return err
				}
				record, err := cmsSetPageDelivery(id, input)
				if err != nil {
					return err
				}
				printJSON(record)
				return nil
			},
		}
		saveCmd.Flags().StringVar(&filePath, "file", "", "Path to delivery JSON")
		saveCmd.MarkFlagRequired("file")
		return saveCmd
	}())
	return cmd
}

func newCMSPageVariantCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "variant", Short: "Localized CMS page variant controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list <page-id>",
		Short: "List localized CMS page variants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			variants, err := cmsListPageVariants(id)
			if err != nil {
				return err
			}
			printJSON(variants)
			return nil
		},
	})
	cmd.AddCommand(newCMSPageVariantSaveCmd(false))
	cmd.AddCommand(newCMSPageVariantSaveCmd(true))
	cmd.AddCommand(newCMSPageVariantTransitionCmd())
	cmd.AddCommand(newCMSPageVariantDeleteCmd())
	return cmd
}

func newCMSPageVariantSaveCmd(update bool) *cobra.Command {
	var filePath string
	use := "create <page-id>"
	short := "Create localized CMS page variant"
	if update {
		use = "update <page-id> <variant-id>"
		short = "Update localized CMS page variant"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if update {
				return cobra.ExactArgs(2)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			var variantID uint
			if update {
				variantID, err = parseCMSID(args[1])
				if err != nil {
					return err
				}
			}
			input, err := loadCMSVariantInput(filePath)
			if err != nil {
				return err
			}
			variant, err := cmsSavePageVariant(pageID, variantID, input)
			if err != nil {
				return err
			}
			printJSON(variant)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to page variant JSON")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCMSPageVariantTransitionCmd() *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "transition <page-id> <variant-id> <submit|approve|request_changes|publish|rollback>",
		Short: "Transition localized CMS page variant workflow",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			variantID, err := parseCMSID(args[1])
			if err != nil {
				return err
			}
			variant, err := cmsTransitionPageVariant(pageID, variantID, args[2], comment)
			if err != nil {
				return err
			}
			printJSON(variant)
			return nil
		},
	}
	cmd.Flags().StringVar(&comment, "comment", "", "Workflow comment")
	return cmd
}

func newCMSPageVariantDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <page-id> <variant-id>",
		Short: "Delete localized CMS page variant",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			variantID, err := parseCMSID(args[1])
			if err != nil {
				return err
			}
			if err := cmsDeletePageVariant(pageID, variantID); err != nil {
				return err
			}
			fmt.Println("✓ CMS page variant deleted")
			return nil
		},
	}
}

func newCMSNavigationListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CMS navigation menus",
		RunE: func(cmd *cobra.Command, args []string) error {
			records, total, err := cmsListNavigation()
			if err != nil {
				return err
			}
			return printCMSList(format, records, total, func(row any) string {
				record := row.(cms.NavigationRecord)
				return fmt.Sprintf("%d\t%s\t%s\t%s\tdraft=%t", record.Menu.ID, record.Menu.Title, record.Menu.Location, record.Entry.Status, record.HasUnpublishedDraft)
			})
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSNavigationGetCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a CMS navigation menu",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsGetNavigation(id)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("%d\t%s\t%s\titems=%d", record.Menu.ID, record.Menu.Title, record.Menu.Location, len(record.Items)))
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSNavigationSaveCmd(update bool) *cobra.Command {
	var filePath, format string
	use := "create"
	short := "Create a CMS navigation draft"
	if update {
		use = "update <id>"
		short = "Update a CMS navigation draft"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if update {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.NoArgs(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := loadCMSNavigationInput(filePath)
			if err != nil {
				return err
			}
			var id uint
			if update {
				id, err = parseCMSID(args[0])
				if err != nil {
					return err
				}
			}
			record, err := cmsSaveNavigation(id, input)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS navigation draft saved: %s (ID: %d)", record.Menu.Title, record.Menu.ID))
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to navigation draft JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCMSNavigationPublishCmd() *cobra.Command {
	var notes, format string
	cmd := &cobra.Command{
		Use:   "publish <id>",
		Short: "Publish a CMS navigation draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsPublishNavigation(id, notes)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS navigation published: %s (ID: %d)", record.Menu.Title, record.Menu.ID))
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Publication notes")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSGlobalListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CMS global regions",
		RunE: func(cmd *cobra.Command, args []string) error {
			records, total, err := cmsListGlobals()
			if err != nil {
				return err
			}
			return printCMSList(format, records, total, func(row any) string {
				record := row.(cms.GlobalRegionRecord)
				return fmt.Sprintf("%d\t%s\t%s\t%s\tdraft=%t", record.Region.ID, record.Region.Title, record.Region.Region, record.Entry.Status, record.HasUnpublishedDraft)
			})
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSGlobalGetCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a CMS global region",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsGetGlobal(id)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("%d\t%s\t%s", record.Region.ID, record.Region.Title, record.Region.Region))
		},
	}
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSGlobalSaveCmd(update bool) *cobra.Command {
	var filePath, format string
	use := "create"
	short := "Create a CMS global region draft"
	if update {
		use = "update <id>"
		short = "Update a CMS global region draft"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if update {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.NoArgs(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var input cms.GlobalRegionDraftInput
			if err := loadCMSGlobalInput(filePath, &input); err != nil {
				return err
			}
			var id uint
			var err error
			if update {
				id, err = parseCMSID(args[0])
				if err != nil {
					return err
				}
			}
			record, err := cmsSaveGlobal(id, input)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS global region draft saved: %s (ID: %d)", record.Region.Title, record.Region.ID))
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to global region draft JSON")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCMSGlobalPublishCmd() *cobra.Command {
	var notes, format string
	cmd := &cobra.Command{
		Use:   "publish <id>",
		Short: "Publish a CMS global region draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			record, err := cmsPublishGlobal(id, notes)
			if err != nil {
				return err
			}
			return printCMSMutation(format, record, fmt.Sprintf("✓ CMS global region published: %s (ID: %d)", record.Region.Title, record.Region.ID))
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Publication notes")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newCMSRedirectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "redirect", Short: "CMS redirect controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List CMS redirects",
		RunE: func(cmd *cobra.Command, args []string) error {
			rules, err := cmsListRedirects()
			if err != nil {
				return err
			}
			printJSON(rules)
			return nil
		},
	})
	cmd.AddCommand(newCMSRedirectSaveCmd(false))
	cmd.AddCommand(newCMSRedirectSaveCmd(true))
	cmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete CMS redirect",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			if err := cmsDeleteRedirect(id); err != nil {
				return err
			}
			fmt.Println("✓ CMS redirect deleted")
			return nil
		},
	})
	return cmd
}

func newCMSRedirectSaveCmd(update bool) *cobra.Command {
	var source, target, matchType string
	var redirectType, priority int
	var enabled bool
	use := "create"
	short := "Create CMS redirect"
	if update {
		use = "update <id>"
		short = "Update CMS redirect"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if update {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.NoArgs(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var id uint
			var err error
			if update {
				id, err = parseCMSID(args[0])
				if err != nil {
					return err
				}
			}
			rule, err := cmsSaveRedirect(id, cms.RedirectInput{
				SourcePattern: source, TargetURL: target, MatchType: matchType,
				RedirectType: redirectType, Priority: priority, IsEnabled: enabled,
			})
			if err != nil {
				return err
			}
			printJSON(rule)
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Source path pattern")
	cmd.Flags().StringVar(&target, "target", "", "Target path or URL")
	cmd.Flags().StringVar(&matchType, "match", "exact", "Match type: exact or prefix")
	cmd.Flags().IntVar(&redirectType, "type", 301, "HTTP redirect type: 301 or 302")
	cmd.Flags().IntVar(&priority, "priority", 0, "Redirect priority")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable redirect")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("target")
	return cmd
}

func newCMSLocaleCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{Use: "locale", Short: "CMS locale controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List CMS locales",
		RunE: func(cmd *cobra.Command, args []string) error {
			locales, err := cmsListLocales()
			if err != nil {
				return err
			}
			printJSON(locales)
			return nil
		},
	})
	saveCmd := &cobra.Command{
		Use:   "set",
		Short: "Update CMS locales from JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := loadCMSLocaleInputs(filePath)
			if err != nil {
				return err
			}
			locales, err := cmsSetLocales(inputs)
			if err != nil {
				return err
			}
			printJSON(locales)
			return nil
		},
	}
	saveCmd.Flags().StringVar(&filePath, "file", "", "Path to locales JSON")
	saveCmd.MarkFlagRequired("file")
	cmd.AddCommand(saveCmd)
	return cmd
}

func newCMSGovernanceCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{Use: "governance", Short: "CMS governance controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get CMS governance settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := cmsGetGovernance()
			if err != nil {
				return err
			}
			printJSON(value)
			return nil
		},
	})
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Update CMS governance settings from JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := loadCMSGovernanceInput(filePath)
			if err != nil {
				return err
			}
			value, err := cmsSetGovernance(input)
			if err != nil {
				return err
			}
			printJSON(value)
			return nil
		},
	}
	setCmd.Flags().StringVar(&filePath, "file", "", "Path to governance JSON")
	setCmd.MarkFlagRequired("file")
	cmd.AddCommand(setCmd)
	return cmd
}

func newCMSAuditCmd() *cobra.Command {
	var entryID uint
	var limit int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List CMS audit events",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := cmsListAudit(entryID, limit)
			if err != nil {
				return err
			}
			printJSON(events)
			return nil
		},
	}
	cmd.Flags().UintVar(&entryID, "entry-id", 0, "Filter by CMS entry ID")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum audit events")
	return cmd
}

func newCMSOperationsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "operations", Short: "CMS operations controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show CMS operations status",
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := cmsOperationsStatus()
			if err != nil {
				return err
			}
			printJSON(value)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "retry-invalidation <id>",
		Short: "Retry failed CMS invalidation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCMSID(args[0])
			if err != nil {
				return err
			}
			if err := cmsRetryInvalidation(id); err != nil {
				return err
			}
			fmt.Println("✓ CMS invalidation queued")
			return nil
		},
	})
	return cmd
}

func newCMSBootstrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap",
		Short: "Create the default editable CMS site without overwriting existing content",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireLocalMode("CMS bootstrap"); err != nil {
				return err
			}
			db := getDB()
			defer closeDB(db)
			result, err := cms.BootstrapStarterSite(db)
			if err != nil {
				return err
			}
			fmt.Printf("CMS bootstrap complete: %d page(s) created, homepage_upgraded=%t, navigation=%t, footer=%t created.\n", len(result.CreatedPages), result.UpgradedHomepage, result.CreatedNavigation, result.CreatedFooter)
			return nil
		},
	}
}

func newCMSScaffoldCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "scaffold", Short: "Print starter CMS JSON payloads"}
	cmd.AddCommand(cmsScaffoldCommand("page", scaffoldPageDraft()))
	cmd.AddCommand(cmsScaffoldCommand("navigation", scaffoldNavigationDraft()))
	cmd.AddCommand(cmsScaffoldCommand("global-footer", scaffoldGlobalFooterDraft()))
	cmd.AddCommand(cmsScaffoldCommand("seo", scaffoldSEO()))
	cmd.AddCommand(cmsScaffoldCommand("delivery", scaffoldDelivery()))
	cmd.AddCommand(cmsScaffoldCommand("locales", scaffoldLocales()))
	cmd.AddCommand(cmsScaffoldCommand("governance", scaffoldGovernance()))
	cmd.AddCommand(cmsScaffoldCommand("variant", scaffoldVariant()))
	return cmd
}

func cmsScaffoldCommand(name string, value any) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: "Print " + name + " JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			printJSON(value)
			return nil
		},
	}
}

func parseCMSID(raw string) (uint, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid CMS id %q", raw)
	}
	return uint(id), nil
}

func loadCMSPageInput(path string, input *cms.PageDraftInput) error {
	var envelope cmsDraftEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return err
	}
	input.Path = envelope.Path
	input.Slug = envelope.Slug
	input.Title = envelope.Title
	input.TemplateKey = envelope.TemplateKey
	input.Visibility = envelope.Visibility
	input.IsHomepage = envelope.IsHomepage
	input.Payload = envelope.Payload
	input.ChangeSummary = envelope.ChangeSummary
	return nil
}

func loadCMSNavigationInput(path string) (cms.NavigationDraftInput, error) {
	var envelope cmsDraftEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return cms.NavigationDraftInput{}, err
	}
	items := []cms.NavigationItemInput{}
	if len(envelope.Items) > 0 {
		var rawItems []cmsNavigationItemEnvelope
		if err := json.Unmarshal(envelope.Items, &rawItems); err != nil {
			return cms.NavigationDraftInput{}, fmt.Errorf("decode navigation items: %w", err)
		}
		items = make([]cms.NavigationItemInput, 0, len(rawItems))
		for _, item := range rawItems {
			items = append(items, cms.NavigationItemInput{
				ID: item.ID, ParentID: item.ParentID, Label: item.Label, ItemType: item.ItemType, TargetRef: item.TargetRef,
				URL: item.URL, SortOrder: item.SortOrder, IsEnabled: item.IsEnabled,
			})
		}
	}
	return cms.NavigationDraftInput{
		Key: envelope.Key, Title: envelope.Title, Location: envelope.Location, Items: items, ChangeSummary: envelope.ChangeSummary,
	}, nil
}

func loadCMSGlobalInput(path string, input *cms.GlobalRegionDraftInput) error {
	var envelope cmsDraftEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return err
	}
	input.Key = envelope.Key
	input.Title = envelope.Title
	input.Region = envelope.Region
	input.Payload = envelope.Payload
	input.ChangeSummary = envelope.ChangeSummary
	return nil
}

func loadCMSSEOInput(path string) (cms.SEOInput, error) {
	var envelope cmsSEOEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return cms.SEOInput{}, err
	}
	if envelope.Robots == "" {
		envelope.Robots = "index_follow"
	}
	if envelope.TwitterCard == "" {
		envelope.TwitterCard = "summary_large_image"
	}
	return cms.SEOInput{
		Title: envelope.Title, Description: envelope.Description, CanonicalURL: envelope.CanonicalURL,
		Robots: envelope.Robots, OGTitle: envelope.OGTitle, OGDescription: envelope.OGDescription,
		OGImageMediaID: envelope.OGImageMediaID, TwitterCard: envelope.TwitterCard,
		TwitterTitle: envelope.TwitterTitle, TwitterDescription: envelope.TwitterDescription,
		TwitterImageMediaID: envelope.TwitterImageMediaID, JSONLD: envelope.JSONLD,
	}, nil
}

func loadCMSDeliveryInput(path string) (cms.DeliveryInput, error) {
	var envelope cmsDeliveryEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return cms.DeliveryInput{}, err
	}
	input := cms.DeliveryInput{}
	if envelope.Schedule != nil {
		publishAt, err := parseCMSTime(envelope.Schedule.PublishAt)
		if err != nil {
			return cms.DeliveryInput{}, fmt.Errorf("schedule.publish_at: %w", err)
		}
		var unpublishAt *time.Time
		if envelope.Schedule.UnpublishAt != nil && strings.TrimSpace(*envelope.Schedule.UnpublishAt) != "" {
			parsed, err := parseCMSTime(*envelope.Schedule.UnpublishAt)
			if err != nil {
				return cms.DeliveryInput{}, fmt.Errorf("schedule.unpublish_at: %w", err)
			}
			unpublishAt = &parsed
		}
		input.Schedule = &cms.ScheduleInput{PublishAt: publishAt, UnpublishAt: unpublishAt, Timezone: envelope.Schedule.Timezone}
	}
	for _, rule := range envelope.TargetingRules {
		input.TargetingRules = append(input.TargetingRules, cms.TargetingRuleInput{
			TargetingRule: cms.TargetingRule{
				Markets: rule.Markets, DeviceClasses: rule.DeviceClasses, AuthStates: rule.AuthStates,
				Referrers: rule.Referrers, UTMSources: rule.UTMSources, SegmentKeys: rule.SegmentKeys,
			},
			Priority: rule.Priority, IsEnabled: rule.IsEnabled,
		})
	}
	if envelope.Experiment != nil {
		startsAt, err := parseCMSTime(envelope.Experiment.StartsAt)
		if err != nil {
			return cms.DeliveryInput{}, fmt.Errorf("experiment.starts_at: %w", err)
		}
		var endsAt *time.Time
		if envelope.Experiment.EndsAt != nil && strings.TrimSpace(*envelope.Experiment.EndsAt) != "" {
			parsed, err := parseCMSTime(*envelope.Experiment.EndsAt)
			if err != nil {
				return cms.DeliveryInput{}, fmt.Errorf("experiment.ends_at: %w", err)
			}
			endsAt = &parsed
		}
		experiment := cms.ExperimentInput{
			Name: envelope.Experiment.Name, Status: models.CMSExperimentStatus(envelope.Experiment.Status),
			StickyKey: envelope.Experiment.StickyKey, StartsAt: startsAt, EndsAt: endsAt,
		}
		for _, variant := range envelope.Experiment.Variants {
			experiment.Variants = append(experiment.Variants, cms.ExperimentVariantInput{
				Name: variant.Name, VersionID: variant.VersionID, Allocation: variant.Allocation,
			})
		}
		input.Experiment = &experiment
	}
	return input, nil
}

func loadCMSLocaleInputs(path string) ([]cms.LocaleInput, error) {
	var envelope cmsLocaleEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return nil, err
	}
	inputs := make([]cms.LocaleInput, 0, len(envelope.Locales))
	for _, locale := range envelope.Locales {
		inputs = append(inputs, cms.LocaleInput{
			Code: locale.Code, Name: locale.Name, Enabled: locale.Enabled, IsDefault: locale.IsDefault, FallbackLocale: locale.FallbackLocale,
		})
	}
	return inputs, nil
}

func loadCMSVariantInput(path string) (cms.VariantInput, error) {
	var envelope cmsVariantEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return cms.VariantInput{}, err
	}
	return cms.VariantInput{
		Locale: envelope.Locale, Market: envelope.Market, Path: envelope.Path, Slug: envelope.Slug,
		Title: envelope.Title, Payload: envelope.Payload, ChangeSummary: envelope.ChangeSummary,
	}, nil
}

func loadCMSGovernanceInput(path string) (cmsGovernanceEnvelope, error) {
	var envelope cmsGovernanceEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return cmsGovernanceEnvelope{}, err
	}
	return envelope, nil
}

func parseCMSTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return parsed, nil
	}
	parsed, err = time.Parse("2006-01-02 15:04", trimmed)
	if err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD HH:MM")
}

func printCMSList[T any](format string, records []T, total int64, line func(any) string) error {
	selected, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}
	if selected == outputFormatJSON {
		printJSON(map[string]any{"data": records, "total": total})
		return nil
	}
	for _, record := range records {
		fmt.Println(line(record))
	}
	fmt.Printf("total=%d\n", total)
	return nil
}

func printCMSMutation(format string, value any, text string) error {
	selected, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}
	if selected == outputFormatJSON {
		printJSON(value)
		return nil
	}
	fmt.Println(text)
	return nil
}

func cmsExportContent() (any, error) {
	if isRemoteMode() {
		return invokeRemoteJSON[map[string]any](http.MethodGet, "/api/v1/admin/cms/export", nil)
	}
	db := getDB()
	defer closeDB(db)
	pageService := cms.NewPageService(db)
	pages, _, err := pageService.List(10000, 0)
	if err != nil {
		return nil, err
	}
	navigation, _, err := cms.NewNavigationService(db).List(10000, 0)
	if err != nil {
		return nil, err
	}
	global, _, err := cms.NewGlobalRegionService(db).List(10000, 0)
	if err != nil {
		return nil, err
	}
	locales, err := pageService.Locales()
	if err != nil {
		return nil, err
	}
	var variants []models.CMSPageVariant
	if err := db.Order("id ASC").Find(&variants).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"schema_version": 1,
		"exported_at":    time.Now().UTC(),
		"locales":        locales,
		"pages":          cmsExportPages(pages),
		"navigation":     cmsExportNavigationMenus(navigation),
		"global_regions": cmsExportGlobalRegions(global),
		"variants":       cmsExportVariants(variants),
	}, nil
}

func cmsExportPages(records []cms.PageRecord) []cmsExportPage {
	out := make([]cmsExportPage, 0, len(records))
	for _, record := range records {
		out = append(out, cmsExportPage{
			Page: record.Page, Entry: record.Entry, CurrentVersion: cmsExportEntryVersion(record.CurrentVersion),
			PublishedVersion: cmsExportEntryVersion(record.PublishedVersion), LatestPublication: record.LatestPublication,
		})
	}
	return out
}

func cmsExportNavigationMenus(records []cms.NavigationRecord) []cmsExportNavigation {
	out := make([]cmsExportNavigation, 0, len(records))
	for _, record := range records {
		out = append(out, cmsExportNavigation{
			Menu: record.Menu, Entry: record.Entry, Items: record.Items, CurrentVersion: cmsExportEntryVersion(record.CurrentVersion),
			PublishedVersion: cmsExportEntryVersion(record.PublishedVersion), LatestPublication: record.LatestPublication,
		})
	}
	return out
}

func cmsExportGlobalRegions(records []cms.GlobalRegionRecord) []cmsExportGlobalRegion {
	out := make([]cmsExportGlobalRegion, 0, len(records))
	for _, record := range records {
		out = append(out, cmsExportGlobalRegion{
			Region: record.Region, Entry: record.Entry, CurrentVersion: cmsExportEntryVersion(record.CurrentVersion),
			PublishedVersion: cmsExportEntryVersion(record.PublishedVersion), LatestPublication: record.LatestPublication,
		})
	}
	return out
}

func cmsExportEntryVersion(version *models.CMSEntryVersion) *cmsExportVersion {
	if version == nil {
		return nil
	}
	var payload cms.PagePayload
	_ = json.Unmarshal([]byte(version.PayloadJSON), &payload)
	changeSummary := version.ChangeSummary
	return &cmsExportVersion{
		ID: version.ID, EntryID: version.EntryID, VersionNumber: version.VersionNumber, SchemaVersion: version.SchemaVersion,
		Payload: payload, CreatedBy: version.CreatedBy, ChangeSummary: &changeSummary, CreatedAt: version.CreatedAt,
	}
}

func cmsExportVariants(variants []models.CMSPageVariant) []cmsExportVariant {
	out := make([]cmsExportVariant, 0, len(variants))
	for _, variant := range variants {
		var payload cms.PagePayload
		_ = json.Unmarshal([]byte(variant.DraftPayloadJSON), &payload)
		submittedBy := optionalNonEmptyString(variant.SubmittedBy)
		approvedBy := optionalNonEmptyString(variant.ApprovedBy)
		out = append(out, cmsExportVariant{
			ID: variant.ID, PageID: variant.PageID, EntryID: variant.EntryID, Locale: variant.Locale, Market: variant.Market,
			Path: variant.Path, Slug: variant.Slug, Title: variant.Title, Payload: payload, Status: string(variant.Status),
			Revision: variant.Revision, SubmittedBy: submittedBy, ApprovedBy: approvedBy, PublishedAt: variant.PublishedAt,
			CreatedAt: variant.CreatedAt, UpdatedAt: variant.UpdatedAt,
		})
	}
	return out
}

func optionalNonEmptyString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func scaffoldPageDraft() map[string]any {
	return map[string]any{
		"path": "/new-page", "title": "New page", "slug": "new-page", "visibility": "public",
		"payload": map[string]any{"blocks": []map[string]any{
			{"type": "hero", "title": "Page title", "subtitle": "Short supporting text"},
			{"type": "rich_text", "body": "Page body"},
		}},
		"change_summary": "Create page",
	}
}

func scaffoldNavigationDraft() map[string]any {
	return map[string]any{
		"key": "main", "title": "Main navigation", "location": "primary",
		"items": []map[string]any{
			{"label": "Home", "item_type": "page", "target_ref": "/", "sort_order": 0, "is_enabled": true},
			{"id": 1, "label": "Shop", "item_type": "dropdown", "sort_order": 1, "is_enabled": true},
			{"parent_id": 1, "label": "Search", "item_type": "internal", "url": "/search", "sort_order": 0, "is_enabled": true},
		},
		"change_summary": "Update navigation",
	}
}

func scaffoldGlobalFooterDraft() map[string]any {
	return map[string]any{
		"key": "site-footer", "title": "Site footer", "region": "footer",
		"payload": map[string]any{"blocks": []map[string]any{{
			"type": "footer", "brand_name": "Store", "tagline": "", "layout": "columns", "theme": "light",
			"columns": []map[string]any{
				{"title": "Shop", "links": []map[string]any{{"label": "Search", "url": "/search"}}},
				{"title": "Help", "links": []map[string]any{{"label": "Orders", "url": "/orders"}}},
			},
			"social_links": []map[string]any{}, "copyright": "© Store",
		}}},
		"change_summary": "Update footer",
	}
}

func scaffoldSEO() map[string]any {
	return map[string]any{
		"title": "", "description": "", "canonical_url": "", "robots": "index_follow",
		"og_title": "", "og_description": "", "og_image_media_id": nil,
		"twitter_card": "summary_large_image", "twitter_title": "", "twitter_description": "",
		"twitter_image_media_id": nil, "json_ld": []map[string]any{},
	}
}

func scaffoldDelivery() map[string]any {
	return map[string]any{
		"schedule": nil,
		"targeting_rules": []map[string]any{{
			"markets": []string{}, "device_classes": []string{"desktop", "mobile"}, "auth_states": []string{"guest", "authenticated"},
			"referrers": []string{}, "utm_sources": []string{}, "segment_keys": []string{}, "priority": 0, "is_enabled": true,
		}},
		"experiment": nil,
	}
}

func scaffoldLocales() map[string]any {
	return map[string]any{"locales": []map[string]any{
		{"code": "en-US", "name": "English (US)", "enabled": true, "is_default": true, "fallback_locale": ""},
	}}
}

func scaffoldGovernance() map[string]any {
	return map[string]any{
		"approval_required": true, "invalidation_webhook_url": "",
		"roles": []map[string]any{{"subject": "admin@example.com", "role": "publisher"}},
	}
}

func scaffoldVariant() map[string]any {
	return map[string]any{
		"locale": "fr-CA", "market": "", "path": "/fr/new-page", "slug": "new-page", "title": "Localized page",
		"payload":        map[string]any{"blocks": []map[string]any{{"type": "rich_text", "body": "Localized body"}}},
		"change_summary": "Create localized variant",
	}
}

func cmsRestoreContent(payload []byte) error {
	if isRemoteMode() {
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			return err
		}
		_, err := invokeRemoteJSON[map[string]any](http.MethodPost, "/api/v1/admin/cms/export", body)
		return err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	return cms.NewPageService(mediaService.DB, mediaService).RestoreExport(payload, "")
}

func cmsListPages() ([]cms.PageRecord, int64, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, 0, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).List(10000, 0)
}

func cmsGetPage(id uint) (*cms.PageRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).Get(id)
}

func cmsSavePage(id uint, input cms.PageDraftInput) (*cms.PageRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	service := cms.NewPageService(mediaService.DB, mediaService)
	if id == 0 {
		return service.CreateDraft(input)
	}
	return service.UpdateDraft(id, input)
}

func cmsPublishPage(id uint, notes string) (*cms.PageRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	return cms.NewPageService(mediaService.DB, mediaService).Publish(id, cms.PublishInput{Notes: notes})
}

func cmsListNavigation() ([]cms.NavigationRecord, int64, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, 0, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewNavigationService(db).List(10000, 0)
}

func cmsGetNavigation(id uint) (*cms.NavigationRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewNavigationService(db).Get(id)
}

func cmsSaveNavigation(id uint, input cms.NavigationDraftInput) (*cms.NavigationRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	service := cms.NewNavigationService(db)
	if id == 0 {
		return service.CreateDraft(input)
	}
	return service.UpdateDraft(id, input)
}

func cmsPublishNavigation(id uint, notes string) (*cms.NavigationRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewNavigationService(db).Publish(id, cms.PublishInput{Notes: notes})
}

func cmsListGlobals() ([]cms.GlobalRegionRecord, int64, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, 0, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewGlobalRegionService(db).List(10000, 0)
}

func cmsGetGlobal(id uint) (*cms.GlobalRegionRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewGlobalRegionService(db).Get(id)
}

func cmsSaveGlobal(id uint, input cms.GlobalRegionDraftInput) (*cms.GlobalRegionRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	service := cms.NewGlobalRegionService(mediaService.DB, mediaService)
	if id == 0 {
		return service.CreateDraft(input)
	}
	return service.UpdateDraft(id, input)
}

func cmsPublishGlobal(id uint, notes string) (*cms.GlobalRegionRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	return cms.NewGlobalRegionService(mediaService.DB, mediaService).Publish(id, cms.PublishInput{Notes: notes})
}

func cmsGetPageSEO(id uint) (*cms.SEORecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).GetSEO(id)
}

func cmsSetPageSEO(id uint, input cms.SEOInput) (*cms.SEORecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	return cms.NewPageService(mediaService.DB, mediaService).UpdateSEO(id, input)
}

func cmsGetPageDelivery(id uint) (*cms.DeliveryRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).GetDelivery(id)
}

func cmsSetPageDelivery(id uint, input cms.DeliveryInput) (*cms.DeliveryRecord, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).UpdateDelivery(id, input)
}

func cmsListPageVariants(pageID uint) ([]models.CMSPageVariant, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).ListVariants(pageID)
}

func cmsSavePageVariant(pageID, variantID uint, input cms.VariantInput) (*models.CMSPageVariant, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	service := cms.NewPageService(mediaService.DB, mediaService)
	if variantID == 0 {
		return service.CreateVariant(pageID, input)
	}
	return service.UpdateVariant(pageID, variantID, input)
}

func cmsTransitionPageVariant(pageID, variantID uint, action, comment string) (*models.CMSPageVariant, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).TransitionVariant(pageID, variantID, action, "cli", comment)
}

func cmsDeletePageVariant(pageID, variantID uint) error {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return err
	}
	mediaService := newMediaService()
	defer closeMediaService(mediaService)
	return cms.NewPageService(mediaService.DB, mediaService).DeleteVariant(pageID, variantID, "cli")
}

func cmsListRedirects() ([]models.CMSRedirectRule, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewRedirectService(db).List()
}

func cmsSaveRedirect(id uint, input cms.RedirectInput) (*models.CMSRedirectRule, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	service := cms.NewRedirectService(db)
	if id == 0 {
		return service.Create(input)
	}
	return service.Update(id, input)
}

func cmsDeleteRedirect(id uint) error {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewRedirectService(db).Delete(id)
}

func cmsListLocales() ([]models.CMSLocale, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).Locales()
}

func cmsSetLocales(inputs []cms.LocaleInput) ([]models.CMSLocale, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).UpdateLocales(inputs, "cli")
}

func cmsGetGovernance() (map[string]any, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	var settings models.CMSSettings
	if err := db.FirstOrCreate(&settings, models.CMSSettings{ID: 1, ApprovalRequired: true}).Error; err != nil {
		return nil, err
	}
	var roles []models.CMSRoleAssignment
	if err := db.Order("subject ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return map[string]any{"approval_required": settings.ApprovalRequired, "invalidation_webhook_url": settings.InvalidationWebhookURL, "roles": roles}, nil
}

func cmsSetGovernance(input cmsGovernanceEnvelope) (map[string]any, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&models.CMSSettings{ID: 1, ApprovalRequired: input.ApprovalRequired, InvalidationWebhookURL: strings.TrimSpace(input.InvalidationWebhookURL)}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.CMSRoleAssignment{}).Error; err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, assignment := range input.Roles {
			subject := strings.TrimSpace(assignment.Subject)
			if subject == "" || seen[subject] || (assignment.Role != "author" && assignment.Role != "editor" && assignment.Role != "publisher") {
				return fmt.Errorf("roles require unique subjects and a valid CMS role")
			}
			seen[subject] = true
			if err := tx.Create(&models.CMSRoleAssignment{Subject: subject, Role: assignment.Role}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return cmsGetGovernance()
}

func cmsListAudit(entryID uint, limit int) ([]models.CMSAuditEvent, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	return cms.NewPageService(db).AuditEvents(entryID, limit)
}

func cmsOperationsStatus() (map[string]any, error) {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return nil, err
	}
	db := getDB()
	defer closeDB(db)
	var schedules, experiments int64
	if err := db.Model(&models.CMSSchedule{}).Where("status = ?", models.CMSScheduleStatusPending).Count(&schedules).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.CMSExperiment{}).Where("status = ?", models.CMSExperimentStatusActive).Count(&experiments).Error; err != nil {
		return nil, err
	}
	var invalidations []models.CMSInvalidationEvent
	if err := db.Order("created_at DESC, id DESC").Limit(100).Find(&invalidations).Error; err != nil {
		return nil, err
	}
	return map[string]any{"pending_schedules": schedules, "active_experiments": experiments, "invalidations": invalidations}, nil
}

func cmsRetryInvalidation(id uint) error {
	if err := requireLocalMode("CMS CLI controls"); err != nil {
		return err
	}
	db := getDB()
	defer closeDB(db)
	result := db.Model(&models.CMSInvalidationEvent{}).Where("id = ?", id).Updates(map[string]any{"status": "pending", "last_error": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("CMS invalidation event not found")
	}
	return nil
}
