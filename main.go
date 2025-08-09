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

var labelName string
var fixAll bool
var rateLimitDelay int
var maxRetries int

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Fix label hierarchies",
	Long:  `Convert period-separated labels to nested hierarchies. Use --label to fix a specific label or --all to fix all detected labels.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate flags
		if labelName != "" && fixAll {
			log.Fatal("Cannot use both --label and --all flags together")
		}
		if labelName == "" && !fixAll {
			log.Fatal("Must specify either --label or --all flag")
		}

		ops, err := setupOperations()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		if fixAll {
			if err := ops.FixAllLabels(); err != nil {
				log.Fatalf("Fix all failed: %v", err)
			}
		} else {
			if err := ops.FixLabel(labelName); err != nil {
				log.Fatalf("Fix failed: %v", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(fixCmd)
	
	// Fix command flags
	fixCmd.Flags().StringVarP(&labelName, "label", "l", "", "Name of the specific label to fix")
	fixCmd.Flags().BoolVar(&fixAll, "all", false, "Fix all period-separated labels")
	fixCmd.Flags().IntVar(&rateLimitDelay, "rate-limit-delay", 200, "Delay between API calls in milliseconds")
	fixCmd.Flags().IntVar(&maxRetries, "max-retries", 3, "Maximum number of retries for rate-limited requests")
}

func setupOperations() (*operations.Operations, error) {
	fmt.Println("🔐 Authenticating with Gmail...")
	
	gmailService, err := auth.GetGmailService()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	client := gmail.NewClient(gmailService)
	
	// Configure rate limiting
	config := &operations.Config{
		RateLimitDelay: rateLimitDelay,
		MaxRetries:     maxRetries,
	}
	
	ops := operations.NewOperationsWithConfig(client, config)
	
	fmt.Println("✅ Authentication successful!")
	
	return ops, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}