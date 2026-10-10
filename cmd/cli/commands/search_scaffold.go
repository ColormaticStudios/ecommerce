package commands

import (
	"fmt"

	"ecommerce/internal/apicontract"
	"github.com/spf13/cobra"
)

func newSearchScaffoldCmd() *cobra.Command {
	return &cobra.Command{Use: "scaffold <synonyms|typo-profiles|ranking-profiles|merchandising-rules|preview|products>", Short: "Print an editable JSON input example", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateSearchFormat(cmd); err != nil {
			return err
		}
		var value any
		switch args[0] {
		case "synonyms":
			value = apicontract.SearchSynonymSetInput{Name: "footwear", Direction: apicontract.SearchSynonymDirection("bi"), Terms: []string{"sneakers", "trainers"}}
		case "typo-profiles":
			value = apicontract.SearchTypoToleranceProfileInput{Name: "custom"}
		case "ranking-profiles":
			weights := []float64{30, 20, 20, 8, 8, 2, 5, 7, 0, 0}
			value = apicontract.SearchRankingProfileInput{Name: "custom", Weights: apicontract.SearchRankingWeightsInput{TokenCoverage: &weights[0], ExactPhrase: &weights[1], Name: &weights[2], Brand: &weights[3], Attributes: &weights[4], Recency: &weights[5], Availability: &weights[6], Sales: &weights[7], Margin: &weights[8], Conversion: &weights[9]}}
		case "merchandising-rules":
			value = apicontract.SearchMerchandisingRuleInput{Name: "featured product", RuleType: apicontract.SearchMerchandisingRuleInputRuleType("pin"), Action: apicontract.SearchMerchandisingActionInput{Targets: []apicontract.SearchMerchandisingTargetInput{{Position: ptrSearchInt(1)}}}}
		case "preview":
			value = apicontract.SearchPreviewRequest{Filters: &apicontract.SearchPreviewFilters{Q: ptrSearchString("sneakers")}}
		case "products":
			value = apicontract.SearchAdminProductsParams{Q: ptrSearchString("sneakers"), Page: ptrSearchInt(1), Limit: ptrSearchInt(20)}
		default:
			return fmt.Errorf("unknown scaffold %q", args[0])
		}
		return writeJSON(cmd.OutOrStdout(), value)
	}}
}
func ptrSearchInt(value int) *int          { return &value }
func ptrSearchString(value string) *string { return &value }
