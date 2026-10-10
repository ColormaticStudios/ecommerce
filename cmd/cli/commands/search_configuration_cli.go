package commands

import (
	"context"
	"fmt"
	"net/http"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"github.com/spf13/cobra"
)

type searchConfigurationCall func(context.Context, *httpapi.CatalogEndpoints, int, any) (any, error)

type searchConfigurationResource struct {
	name, description                 string
	createBody, updateBody            func() any
	list, get, create, update, delete searchConfigurationCall
	merchandising                     bool
}

func newSearchConfigurationCommands() []*cobra.Command {
	resources := []searchConfigurationResource{
		{name: "synonyms", description: "synonym sets", merchandising: false, createBody: func() any { return &apicontract.SearchSynonymSetInput{} }, updateBody: func() any { return &apicontract.SearchSynonymSetPatch{} },
			list: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.ListAdminSearchSynonyms(ctx, apicontract.ListAdminSearchSynonymsRequestObject{})
			},
			get: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.GetAdminSearchSynonym(ctx, apicontract.GetAdminSearchSynonymRequestObject{Id: id})
			},
			create: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.CreateAdminSearchSynonym(ctx, apicontract.CreateAdminSearchSynonymRequestObject{Body: body.(*apicontract.SearchSynonymSetInput)})
			},
			update: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.UpdateAdminSearchSynonym(ctx, apicontract.UpdateAdminSearchSynonymRequestObject{Id: id, Body: body.(*apicontract.SearchSynonymSetPatch)})
			},
			delete: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.DeleteAdminSearchSynonym(ctx, apicontract.DeleteAdminSearchSynonymRequestObject{Id: id})
			},
		},
		{name: "typo-profiles", description: "typo tolerance profiles", merchandising: false, createBody: func() any { return &apicontract.SearchTypoToleranceProfileInput{} }, updateBody: func() any { return &apicontract.SearchTypoToleranceProfilePatch{} },
			list: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.ListAdminSearchTypoProfiles(ctx, apicontract.ListAdminSearchTypoProfilesRequestObject{})
			},
			get: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.GetAdminSearchTypoProfile(ctx, apicontract.GetAdminSearchTypoProfileRequestObject{Id: id})
			},
			create: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.CreateAdminSearchTypoProfile(ctx, apicontract.CreateAdminSearchTypoProfileRequestObject{Body: body.(*apicontract.SearchTypoToleranceProfileInput)})
			},
			update: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.UpdateAdminSearchTypoProfile(ctx, apicontract.UpdateAdminSearchTypoProfileRequestObject{Id: id, Body: body.(*apicontract.SearchTypoToleranceProfilePatch)})
			},
			delete: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.DeleteAdminSearchTypoProfile(ctx, apicontract.DeleteAdminSearchTypoProfileRequestObject{Id: id})
			},
		},
		{name: "ranking-profiles", description: "ranking profiles", merchandising: false, createBody: func() any { return &apicontract.SearchRankingProfileInput{} }, updateBody: func() any { return &apicontract.SearchRankingProfilePatch{} },
			list: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.ListAdminSearchRankingProfiles(ctx, apicontract.ListAdminSearchRankingProfilesRequestObject{})
			},
			get: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.GetAdminSearchRankingProfile(ctx, apicontract.GetAdminSearchRankingProfileRequestObject{Id: id})
			},
			create: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.CreateAdminSearchRankingProfile(ctx, apicontract.CreateAdminSearchRankingProfileRequestObject{Body: body.(*apicontract.SearchRankingProfileInput)})
			},
			update: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.UpdateAdminSearchRankingProfile(ctx, apicontract.UpdateAdminSearchRankingProfileRequestObject{Id: id, Body: body.(*apicontract.SearchRankingProfilePatch)})
			},
			delete: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.DeleteAdminSearchRankingProfile(ctx, apicontract.DeleteAdminSearchRankingProfileRequestObject{Id: id})
			},
		},
		{name: "merchandising-rules", description: "merchandising rules", merchandising: true, createBody: func() any { return &apicontract.SearchMerchandisingRuleInput{} }, updateBody: func() any { return &apicontract.SearchMerchandisingRulePatch{} },
			list: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.ListAdminSearchMerchandisingRules(ctx, apicontract.ListAdminSearchMerchandisingRulesRequestObject{})
			},
			get: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.GetAdminSearchMerchandisingRule(ctx, apicontract.GetAdminSearchMerchandisingRuleRequestObject{Id: id})
			},
			create: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.CreateAdminSearchMerchandisingRule(ctx, apicontract.CreateAdminSearchMerchandisingRuleRequestObject{Body: body.(*apicontract.SearchMerchandisingRuleInput)})
			},
			update: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.UpdateAdminSearchMerchandisingRule(ctx, apicontract.UpdateAdminSearchMerchandisingRuleRequestObject{Id: id, Body: body.(*apicontract.SearchMerchandisingRulePatch)})
			},
			delete: func(ctx context.Context, e *httpapi.CatalogEndpoints, id int, body any) (any, error) {
				return e.DeleteAdminSearchMerchandisingRule(ctx, apicontract.DeleteAdminSearchMerchandisingRuleRequestObject{Id: id})
			},
		},
	}
	commands := make([]*cobra.Command, 0, len(resources)+2)
	for _, resource := range resources {
		commands = append(commands, newSearchConfigurationResource(resource))
	}
	return append(commands, newSearchMerchandisingAuditCmd(), newSearchPreviewCmd())
}

func newSearchConfigurationResource(resource searchConfigurationResource) *cobra.Command {
	group := &cobra.Command{Use: resource.name, Short: "Manage search " + resource.description}
	basePath := "/api/v1/admin/search/" + resource.name
	list := &cobra.Command{Use: "list", Short: "List " + resource.description, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			value, err := searchCLIRequest(cmd, http.MethodGet, basePath, nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
				return resource.list(ctx, e, 0, nil)
			})
			if err != nil {
				return err
			}
			return writeSearchOutput(cmd, value)
		}}
	get := &cobra.Command{Use: "get <id>", Short: "Get a search configuration", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := searchIDArg(args)
		if err != nil {
			return err
		}
		value, err := searchCLIRequest(cmd, http.MethodGet, fmt.Sprintf("%s/%d", basePath, id), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return resource.get(ctx, e, id, nil)
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	group.AddCommand(list, get)
	for _, update := range []bool{false, true} {
		use, method, short := "create", http.MethodPost, "Create a search configuration"
		args := cobra.NoArgs
		if update {
			use = "update <id>"
			method = http.MethodPatch
			short = "Update a search configuration"
			args = cobra.ExactArgs(1)
		}
		var file, productQuery string
		command := &cobra.Command{Use: use, Short: short, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
			id := 0
			path := basePath
			makeBody, call := resource.createBody, resource.create
			if update {
				var err error
				id, err = searchIDArg(args)
				if err != nil {
					return err
				}
				path = fmt.Sprintf("%s/%d", basePath, id)
				makeBody, call = resource.updateBody, resource.update
			}
			body := makeBody()
			if err := readSearchInput(cmd, file, body); err != nil {
				return err
			}
			if resource.merchandising && cmd.Flags().Changed("product-query") {
				var action *apicontract.SearchMerchandisingActionInput
				switch input := body.(type) {
				case *apicontract.SearchMerchandisingRuleInput:
					action = &input.Action
				case *apicontract.SearchMerchandisingRulePatch:
					action = input.Action
				}
				if err := resolveSearchMerchandisingTarget(cmd, action, productQuery); err != nil {
					return err
				}
			}
			value, err := searchCLIRequest(cmd, method, path, body, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) { return call(ctx, e, id, body) })
			if err != nil {
				return err
			}
			return writeSearchOutput(cmd, value)
		}}
		command.Flags().StringVar(&file, "file", "", "JSON input file, or - for stdin")
		_ = command.MarkFlagRequired("file")
		if resource.merchandising {
			command.Flags().StringVar(&productQuery, "product-query", "", "Look up one action target by product name or search query; omit its product_id")
		}
		group.AddCommand(command)
	}
	deleteCommand := &cobra.Command{Use: "delete <id>", Short: "Delete a search configuration", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := searchIDArg(args)
		if err != nil {
			return err
		}
		if err := confirmSearchDelete(cmd, id); err != nil {
			return err
		}
		_, err = searchCLIRequest(cmd, http.MethodDelete, fmt.Sprintf("%s/%d", basePath, id), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return resource.delete(ctx, e, id, nil)
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, map[string]any{"id": id, "deleted": true})
	}}
	deleteCommand.Flags().Bool("yes", false, "Confirm deletion without prompting")
	group.AddCommand(deleteCommand)
	if resource.merchandising {
		group.AddCommand(newSearchMerchandisingRuleAuditCmd())
	}
	return group
}

func resolveSearchMerchandisingTarget(cmd *cobra.Command, action *apicontract.SearchMerchandisingActionInput, query string) error {
	if action == nil || len(action.Targets) != 1 {
		return fmt.Errorf("--product-query requires an action with exactly one target")
	}
	if action.Targets[0].ProductId != nil {
		return fmt.Errorf("--product-query cannot be combined with an explicit product_id")
	}
	id, err := resolveSearchProduct(cmd, query)
	if err != nil {
		return err
	}
	action.Targets[0].ProductId = &id
	return nil
}

func newSearchMerchandisingRuleAuditCmd() *cobra.Command {
	return &cobra.Command{Use: "audit <id>", Short: "List immutable audit history for a rule, including deleted rules", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := searchIDArg(args)
		if err != nil {
			return err
		}
		value, err := searchCLIRequest(cmd, http.MethodGet, fmt.Sprintf("/api/v1/admin/search/merchandising-rules/%d/audit", id), nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.ListAdminSearchMerchandisingRuleAudit(ctx, apicontract.ListAdminSearchMerchandisingRuleAuditRequestObject{Id: id})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
}

func newSearchMerchandisingAuditCmd() *cobra.Command {
	return &cobra.Command{Use: "merchandising-audit", Short: "List all immutable merchandising audit history", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		value, err := searchCLIRequest(cmd, http.MethodGet, "/api/v1/admin/search/merchandising-audit", nil, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.ListAdminSearchMerchandisingAudit(ctx, apicontract.ListAdminSearchMerchandisingAuditRequestObject{})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
}

func newSearchPreviewCmd() *cobra.Command {
	var file string
	command := &cobra.Command{Use: "preview", Short: "Compare baseline and proposed search results without saving changes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var body apicontract.SearchPreviewRequest
		if err := readSearchInput(cmd, file, &body); err != nil {
			return err
		}
		value, err := searchCLIRequest(cmd, http.MethodPost, "/api/v1/admin/search/preview", &body, func(ctx context.Context, e *httpapi.CatalogEndpoints) (any, error) {
			return e.PreviewAdminSearch(ctx, apicontract.PreviewAdminSearchRequestObject{Body: &body})
		})
		if err != nil {
			return err
		}
		return writeSearchOutput(cmd, value)
	}}
	command.Flags().StringVar(&file, "file", "", "JSON preview input file, or - for stdin")
	_ = command.MarkFlagRequired("file")
	return command
}
