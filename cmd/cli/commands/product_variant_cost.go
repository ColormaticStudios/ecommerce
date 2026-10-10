package commands

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"

	"ecommerce/internal/apicontract"
	"github.com/spf13/cobra"
)

type variantCostProductAccess struct {
	get    func(context.Context, int) (apicontract.Product, error)
	update func(context.Context, int, apicontract.ProductUpsertInput) (apicontract.Product, error)
}

func newVariantCostCmd() *cobra.Command {
	return newVariantCostCmdWithAccess(variantCostProductAccess{
		get: func(ctx context.Context, id int) (apicontract.Product, error) {
			if isRemoteMode() {
				return invokeCLIJSON[apicontract.Product](ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/products/%d", id), nil)
			}
			e, close, err := openCLICatalogEndpoints(ctx)
			if err != nil {
				return apicontract.Product{}, err
			}
			defer close()
			response, err := e.GetAdminProduct(ctx, apicontract.GetAdminProductRequestObject{Id: id})
			if err != nil {
				return apicontract.Product{}, err
			}
			value, ok := response.(apicontract.GetAdminProduct200JSONResponse)
			if !ok {
				return apicontract.Product{}, fmt.Errorf("unexpected product response %T", response)
			}
			return apicontract.Product(value), nil
		},
		update: func(ctx context.Context, id int, input apicontract.ProductUpsertInput) (apicontract.Product, error) {
			if isRemoteMode() {
				return invokeCLIJSON[apicontract.Product](ctx, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d", id), input)
			}
			e, close, err := openCLICatalogEndpoints(ctx)
			if err != nil {
				return apicontract.Product{}, err
			}
			defer close()
			response, err := e.UpdateProduct(ctx, apicontract.UpdateProductRequestObject{Id: id, Body: &input})
			if err != nil {
				return apicontract.Product{}, err
			}
			value, ok := response.(apicontract.UpdateProduct200JSONResponse)
			if !ok {
				return apicontract.Product{}, fmt.Errorf("unexpected product update response %T", response)
			}
			return apicontract.Product(value), nil
		},
	})
}

func newVariantCostCmdWithAccess(access variantCostProductAccess) *cobra.Command {
	var id int
	var sku, format string
	var cost float64
	var clear bool
	cmd := &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Use:           "variant-cost", Short: "Read or edit a private variant unit cost in the product draft",
		Long: "Select a product by ID and a variant by its exact SKU. Without --unit-cost or --clear, read its current administrator cost. Edits save a draft; publish the product separately to refresh search margin scoring.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if id < 1 {
				return fmt.Errorf("product ID must be positive")
			}
			sku = strings.TrimSpace(sku)
			if sku == "" {
				return fmt.Errorf("variant SKU is required")
			}
			set := cmd.Flags().Changed("unit-cost")
			if cmd.Flags().Changed("clear") && !clear {
				return fmt.Errorf("use --clear to remove the unit cost")
			}
			if set && clear {
				return fmt.Errorf("--unit-cost and --clear cannot be used together")
			}
			if set && (math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 || cost > 9999999999.99) {
				return fmt.Errorf("unit cost must be a finite amount between zero and 9999999999.99")
			}
			product, err := access.get(cmd.Context(), id)
			if err != nil {
				return err
			}
			input := productContractToUpsertInput(product)
			index := -1
			for i, variant := range input.Variants {
				if variant.Sku == sku {
					if index != -1 {
						return fmt.Errorf("variant SKU %q is ambiguous", sku)
					}
					index = i
				}
			}
			if index == -1 {
				return fmt.Errorf("variant SKU %q does not exist on product %d", sku, id)
			}
			if set || clear {
				input.Variants[index].UnitCost = nil
				if set {
					input.Variants[index].UnitCost = &cost
				}
				product, err = access.update(cmd.Context(), id, input)
				if err != nil {
					return err
				}
			}
			if selectedFormat == outputFormatJSON {
				return writeJSON(cmd.OutOrStdout(), product)
			}
			for _, variant := range product.Variants {
				if variant.Sku == sku {
					if variant.UnitCost == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Product %d variant %s unit cost: unknown\n", id, sku)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Product %d variant %s unit cost: %.2f\n", id, sku, *variant.UnitCost)
					}
				}
			}
			if set || clear {
				fmt.Fprintln(cmd.OutOrStdout(), "Saved to draft; publish the product to apply the cost.")
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&id, "id", "i", 0, "Product ID")
	cmd.Flags().StringVar(&sku, "variant-sku", "", "Exact variant SKU")
	cmd.Flags().Float64Var(&cost, "unit-cost", 0, "Private unit cost, including zero")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the private unit cost to unknown")
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("variant-sku")
	return cmd
}
