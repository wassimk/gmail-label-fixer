package main

import (
	"fmt"
	"log"
	"os"

	"gmail-label-fixer/internal/auth"
	"gmail-label-fixer/internal/gmail"
	"gmail-label-fixer/internal/operations"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gmail-label-fixer",
	Short: "Fix Gmail label hierarchies from period-separated to nested format",
	Long:  `A CLI tool to convert period-separated Gmail labels (like Vacations.2025.Mexico) into properly nested label hierarchies (Vacations/2025/Mexico).`,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze existing labels and show proposed changes (dry run)",
	Long:  `Scan all Gmail labels and identify period-separated labels that can be converted to nested hierarchies. Shows what changes would be made without actually making them.`,
	Run: func(cmd *cobra.Command, args []string) {
		ops, err := setupOperations()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		if err := ops.DryRun(); err != nil {
			log.Fatalf("Analysis failed: %v", err)
		}
	},
}

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Fix label hierarchies",
	Long:  `Convert period-separated labels to nested hierarchies. Use --label to fix specific labels or --all to fix all detected labels.`,
}

var labelName string
var fixAll bool

var fixLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Fix a specific label",
	Long:  `Fix a specific period-separated label and convert it to a nested hierarchy.`,
	Run: func(cmd *cobra.Command, args []string) {
		if labelName == "" {
			log.Fatal("Label name is required. Use --label flag")
		}

		ops, err := setupOperations()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		if err := ops.FixLabel(labelName); err != nil {
			log.Fatalf("Fix failed: %v", err)
		}
	},
}

var fixAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Fix all period-separated labels",
	Long:  `Convert all detected period-separated labels to nested hierarchies.`,
	Run: func(cmd *cobra.Command, args []string) {
		ops, err := setupOperations()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		if err := ops.FixAllLabels(); err != nil {
			log.Fatalf("Fix all failed: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(fixCmd)
	
	fixCmd.AddCommand(fixLabelCmd)
	fixCmd.AddCommand(fixAllCmd)
	
	fixLabelCmd.Flags().StringVarP(&labelName, "label", "l", "", "Name of the label to fix")
	fixLabelCmd.MarkFlagRequired("label")
}

func setupOperations() (*operations.Operations, error) {
	fmt.Println("🔐 Authenticating with Gmail...")
	
	gmailService, err := auth.GetGmailService()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	client := gmail.NewClient(gmailService)
	ops := operations.NewOperations(client)
	
	fmt.Println("✅ Authentication successful!")
	
	return ops, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}