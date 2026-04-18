package commands

import (
	"errors"
	"fmt"

	accountdataservice "ecommerce/internal/services/accountdata"

	"github.com/spf13/cobra"
)

func newUserAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Manage saved addresses for a user",
	}

	cmd.AddCommand(newListUserAddressesCmd())
	cmd.AddCommand(newAddUserAddressCmd())
	cmd.AddCommand(newDeleteUserAddressCmd())
	cmd.AddCommand(newDefaultUserAddressCmd())

	return cmd
}

func newUserCardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "card",
		Aliases: []string{"payment-method"},
		Short:   "Manage saved payment cards for a user",
	}

	cmd.AddCommand(newListUserCardsCmd())
	cmd.AddCommand(newAddUserCardCmd())
	cmd.AddCommand(newDeleteUserCardCmd())
	cmd.AddCommand(newDefaultUserCardCmd())

	return cmd
}

func newListUserAddressesCmd() *cobra.Command {
	var userID uint
	var email, username, format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved addresses for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			addresses, err := accountdataservice.NewService(db).ListSavedAddresses(user.ID)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(addresses)
				return nil
			}

			if len(addresses) == 0 {
				fmt.Println("No saved addresses found")
				return nil
			}

			fmt.Printf("%-5s %-20s %-24s %-10s\n", "ID", "Label", "City", "Default")
			fmt.Println("----------------------------------------------------------")
			for _, address := range addresses {
				fmt.Printf("%-5d %-20s %-24s %-10t\n", address.ID, address.Label, address.City, address.IsDefault)
			}
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newAddUserAddressCmd() *cobra.Command {
	var userID uint
	var email, username, format string
	var input savedAddressInput

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a saved address to a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			address, err := accountdataservice.NewService(db).CreateSavedAddress(user.ID, accountdataservice.CreateSavedAddressInput{
				Label:      input.label,
				FullName:   input.fullName,
				Line1:      input.line1,
				Line2:      input.line2,
				City:       input.city,
				State:      input.state,
				PostalCode: input.postalCode,
				Country:    input.country,
				Phone:      input.phone,
				SetDefault: input.setDefault,
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(address)
				return nil
			}

			fmt.Printf("✓ Address added for %s: %s (ID: %d)\n", user.Username, address.Label, address.ID)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	input.bindAddress(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("full-name")
	cmd.MarkFlagRequired("line1")
	cmd.MarkFlagRequired("city")
	cmd.MarkFlagRequired("postal-code")
	cmd.MarkFlagRequired("country")
	return cmd
}

func newDeleteUserAddressCmd() *cobra.Command {
	var userID, addressID uint
	var email, username string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a saved address",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			if err := accountdataservice.NewService(db).DeleteSavedAddress(user.ID, addressID); err != nil {
				if err.Error() == "address not found" {
					return errors.New("saved address not found")
				}
				return err
			}

			fmt.Printf("✓ Address %d deleted for %s\n", addressID, user.Username)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	cmd.Flags().UintVar(&addressID, "id", 0, "Saved address ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

func newDefaultUserAddressCmd() *cobra.Command {
	var userID, addressID uint
	var email, username string

	cmd := &cobra.Command{
		Use:   "set-default",
		Short: "Set a saved address as default",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			if _, err := accountdataservice.NewService(db).SetDefaultAddress(user.ID, addressID); err != nil {
				if err.Error() == "address not found" {
					return errors.New("saved address not found")
				}
				return err
			}

			fmt.Printf("✓ Address %d is now default for %s\n", addressID, user.Username)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	cmd.Flags().UintVar(&addressID, "id", 0, "Saved address ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

func newListUserCardsCmd() *cobra.Command {
	var userID uint
	var email, username, format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved payment cards for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			methods, err := accountdataservice.NewService(db).ListSavedPaymentMethods(user.ID)
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(methods)
				return nil
			}

			if len(methods) == 0 {
				fmt.Println("No saved payment cards found")
				return nil
			}

			fmt.Printf("%-5s %-15s %-8s %-8s %-10s\n", "ID", "Brand", "Last4", "Default", "Expires")
			fmt.Println("------------------------------------------------------")
			for _, method := range methods {
				fmt.Printf("%-5d %-15s %-8s %-8t %02d/%d\n", method.ID, method.Brand, method.Last4, method.IsDefault, method.ExpMonth, method.ExpYear)
			}
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	return cmd
}

func newAddUserCardCmd() *cobra.Command {
	var userID uint
	var email, username, format string
	var input savedCardInput

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a saved payment card to a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			method, err := accountdataservice.NewService(db).CreateSavedPaymentMethod(user.ID, accountdataservice.CreateSavedPaymentMethodInput{
				CardholderName: input.cardholderName,
				CardNumber:     input.cardNumber,
				ExpMonth:       input.expMonth,
				ExpYear:        input.expYear,
				Nickname:       input.nickname,
				SetDefault:     input.setDefault,
			})
			if err != nil {
				return err
			}

			selectedFormat, err := normalizeOutputFormat(format)
			if err != nil {
				return err
			}
			if selectedFormat == outputFormatJSON {
				printJSON(method)
				return nil
			}

			fmt.Printf("✓ Card added for %s: %s •••• %s (ID: %d)\n", user.Username, method.Brand, method.Last4, method.ID)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	input.bind(cmd)
	addOutputFormatFlag(cmd, &format, string(outputFormatText))
	cmd.MarkFlagRequired("cardholder-name")
	cmd.MarkFlagRequired("card-number")
	cmd.MarkFlagRequired("exp-month")
	cmd.MarkFlagRequired("exp-year")
	return cmd
}

func newDeleteUserCardCmd() *cobra.Command {
	var userID, methodID uint
	var email, username string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a saved payment card",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			if err := accountdataservice.NewService(db).DeleteSavedPaymentMethod(user.ID, methodID); err != nil {
				if err.Error() == "payment method not found" {
					return errors.New("saved payment card not found")
				}
				return err
			}

			fmt.Printf("✓ Card %d deleted for %s\n", methodID, user.Username)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	cmd.Flags().UintVar(&methodID, "id", 0, "Saved payment card ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

func newDefaultUserCardCmd() *cobra.Command {
	var userID, methodID uint
	var email, username string

	cmd := &cobra.Command{
		Use:   "set-default",
		Short: "Set a saved payment card as default",
		RunE: func(cmd *cobra.Command, args []string) error {
			db := getDB()
			defer closeDB(db)

			user, err := requireUser(db, userID, email, username)
			if err != nil {
				return err
			}

			if _, err := accountdataservice.NewService(db).SetDefaultPaymentMethod(user.ID, methodID); err != nil {
				if err.Error() == "payment method not found" {
					return errors.New("saved payment card not found")
				}
				return err
			}

			fmt.Printf("✓ Card %d is now default for %s\n", methodID, user.Username)
			return nil
		},
	}

	addUserSelectorFlags(cmd, &userID, &email, &username)
	cmd.Flags().UintVar(&methodID, "id", 0, "Saved payment card ID")
	cmd.MarkFlagRequired("id")
	return cmd
}

type savedAddressInput struct {
	label      string
	fullName   string
	line1      string
	line2      string
	city       string
	state      string
	postalCode string
	country    string
	phone      string
	setDefault bool
}

func (i *savedAddressInput) bindAddress(cmd *cobra.Command) {
	cmd.Flags().StringVar(&i.label, "label", "", "Address label")
	cmd.Flags().StringVar(&i.fullName, "full-name", "", "Full name")
	cmd.Flags().StringVar(&i.line1, "line1", "", "Address line 1")
	cmd.Flags().StringVar(&i.line2, "line2", "", "Address line 2")
	cmd.Flags().StringVar(&i.city, "city", "", "City")
	cmd.Flags().StringVar(&i.state, "state", "", "State/region")
	cmd.Flags().StringVar(&i.postalCode, "postal-code", "", "Postal code")
	cmd.Flags().StringVar(&i.country, "country", "", "Two-letter country code")
	cmd.Flags().StringVar(&i.phone, "phone", "", "Phone number")
	cmd.Flags().BoolVar(&i.setDefault, "set-default", false, "Set as the default saved address")
}

type savedCardInput struct {
	cardholderName string
	cardNumber     string
	expMonth       int
	expYear        int
	nickname       string
	setDefault     bool
}

func (i *savedCardInput) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&i.cardholderName, "cardholder-name", "", "Cardholder name")
	cmd.Flags().StringVar(&i.cardNumber, "card-number", "", "Card number")
	cmd.Flags().IntVar(&i.expMonth, "exp-month", 0, "Expiration month")
	cmd.Flags().IntVar(&i.expYear, "exp-year", 0, "Expiration year")
	cmd.Flags().StringVar(&i.nickname, "nickname", "", "Card nickname")
	cmd.Flags().BoolVar(&i.setDefault, "set-default", false, "Set as the default saved card")
}
