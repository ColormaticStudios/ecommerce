package commands

import (
	"context"
	"net/http"

	"ecommerce/internal/apicontract"
	localizationservice "ecommerce/internal/services/localization"

	"github.com/spf13/cobra"
)

type localizationLocaleEnvelope struct {
	Locales []struct {
		Code           string   `json:"code"`
		Name           string   `json:"name"`
		IsEnabled      bool     `json:"is_enabled"`
		IsDefault      bool     `json:"is_default"`
		FallbackLocale string   `json:"fallback_locale"`
		DefaultMarkets []string `json:"default_for_markets"`
	} `json:"locales"`
}

func NewLocalizationCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{Use: "localization", Short: "Localization platform controls"}
	cmd.AddCommand(&cobra.Command{
		Use:   "locales",
		Short: "List platform locales",
		RunE: func(cmd *cobra.Command, _ []string) error {
			locales, err := listLocalizationLocales(cmd.Context())
			if err != nil {
				return err
			}
			printJSON(locales)
			return nil
		},
	})
	setCmd := &cobra.Command{
		Use:   "set-locales",
		Short: "Replace platform locales from JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			inputs, err := loadLocalizationLocaleInputs(filePath)
			if err != nil {
				return err
			}
			locales, err := replaceLocalizationLocales(cmd.Context(), inputs)
			if err != nil {
				return err
			}
			printJSON(locales)
			return nil
		},
	}
	setCmd.Flags().StringVar(&filePath, "file", "", "Path to locale registry JSON")
	setCmd.MarkFlagRequired("file")
	cmd.AddCommand(setCmd)
	return cmd
}

func loadLocalizationLocaleInputs(path string) ([]localizationservice.LocaleInput, error) {
	var envelope localizationLocaleEnvelope
	if err := loadJSONFile(path, &envelope); err != nil {
		return nil, err
	}
	inputs := make([]localizationservice.LocaleInput, 0, len(envelope.Locales))
	for _, locale := range envelope.Locales {
		inputs = append(inputs, localizationservice.LocaleInput{
			Code: locale.Code, Name: locale.Name, IsEnabled: locale.IsEnabled,
			IsDefault: locale.IsDefault, FallbackLocale: locale.FallbackLocale, DefaultMarkets: locale.DefaultMarkets,
		})
	}
	return inputs, nil
}

func listLocalizationLocales(ctx context.Context) (any, error) {
	if isRemoteMode() {
		return invokeRemoteJSON[apicontract.LocalizationLocaleList](http.MethodGet, "/api/v1/admin/localization/locales", nil)
	}
	db := getDB()
	defer closeDB(db)
	return localizationservice.NewService(db).ListLocales(ctx, false)
}

func replaceLocalizationLocales(ctx context.Context, inputs []localizationservice.LocaleInput) (any, error) {
	if isRemoteMode() {
		body := apicontract.LocalizationLocaleSettingsInput{Locales: make([]apicontract.LocalizationLocaleInput, 0, len(inputs))}
		for _, input := range inputs {
			fallback := input.FallbackLocale
			var fallbackPointer *string
			if fallback != "" {
				fallbackPointer = &fallback
			}
			body.Locales = append(body.Locales, apicontract.LocalizationLocaleInput{
				Code: input.Code, Name: input.Name, IsEnabled: input.IsEnabled,
				IsDefault: input.IsDefault, FallbackLocale: fallbackPointer, DefaultForMarkets: input.DefaultMarkets,
			})
		}
		return invokeRemoteJSON[apicontract.LocalizationLocaleList](http.MethodPut, "/api/v1/admin/localization/locales", body)
	}
	db := getDB()
	defer closeDB(db)
	return localizationservice.NewService(db).ReplaceLocales(ctx, inputs)
}
