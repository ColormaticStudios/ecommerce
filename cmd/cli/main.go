package main

import (
	"ecommerce/cmd/cli/commands"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ecommerce-cli",
		Short: "Ecommerce API CLI tool for administrative tasks",
		Long:  "A command-line tool for managing users, products, and other administrative tasks for the ecommerce API.",
	}

	// Add subcommands
	rootCmd.AddCommand(commands.NewUserCmd())
	rootCmd.AddCommand(commands.NewProductCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
