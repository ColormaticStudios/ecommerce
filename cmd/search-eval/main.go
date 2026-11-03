package main

import (
	"context"
	"ecommerce/internal/search"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	report, err := search.EvaluateRelevance(context.Background())
	if err == nil {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(report)
	}
	if err == nil {
		err = report.CheckRegression()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
