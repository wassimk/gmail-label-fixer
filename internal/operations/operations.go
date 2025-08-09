package operations

import (
	"fmt"
	"gmail-label-fixer/internal/analyzer"
	"gmail-label-fixer/internal/gmail"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	gmailAPI "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

const (
	defaultRateLimitDelay = 200  // Default delay between API calls in milliseconds
	defaultMaxRetries     = 3    // Default maximum retries for rate-limited requests
	maxBackoffDelay       = 32   // Maximum backoff delay in seconds
	jitterMaxMs           = 1000 // Maximum jitter in milliseconds
)

type Config struct {
	RateLimitDelay int // Delay between API calls in milliseconds
	MaxRetries     int // Maximum retries for rate-limited requests
}

type Operations struct {
	client   *gmail.Client
	analyzer *analyzer.Analyzer
	config   *Config
}

func NewOperations(client *gmail.Client) *Operations {
	return &Operations{
		client:   client,
		analyzer: analyzer.NewAnalyzer(client),
		config: &Config{
			RateLimitDelay: defaultRateLimitDelay,
			MaxRetries:     defaultMaxRetries,
		},
	}
}

func NewOperationsWithConfig(client *gmail.Client, config *Config) *Operations {
	return &Operations{
		client:   client,
		analyzer: analyzer.NewAnalyzer(client),
		config:   config,
	}
}

// withRateLimit applies rate limiting delay between operations
func (o *Operations) withRateLimit() {
	if o.config.RateLimitDelay > 0 {
		time.Sleep(time.Duration(o.config.RateLimitDelay) * time.Millisecond)
	}
}

// retryWithBackoff performs an operation with exponential backoff for rate limits
func (o *Operations) retryWithBackoff(operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= o.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			baseDelay := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			jitter := time.Duration(rand.Intn(jitterMaxMs)) * time.Millisecond
			delay := baseDelay + jitter

			if delay > maxBackoffDelay*time.Second {
				delay = maxBackoffDelay * time.Second // Cap at maximum backoff delay
			}

			fmt.Printf("   ⏳ Rate limit hit, waiting %v before retry %d/%d...\n", delay, attempt, o.config.MaxRetries)
			time.Sleep(delay)
		}

		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if this is a retryable error
		if !isRetryableError(err) {
			return err // Don't retry non-retryable errors
		}

		// Don't sleep after the last attempt
		if attempt == o.config.MaxRetries {
			break
		}
	}

	return fmt.Errorf("operation failed after %d retries: %v", o.config.MaxRetries, lastErr)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Google API errors
	if apiErr, ok := err.(*googleapi.Error); ok {
		switch apiErr.Code {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		case http.StatusForbidden: // 403 - might be quota exceeded
			return strings.Contains(strings.ToLower(apiErr.Message), "quota") ||
				strings.Contains(strings.ToLower(apiErr.Message), "rate limit")
		}
	}

	// Check for network-related errors
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "temporary failure")
}

func (o *Operations) DryRun() error {
	fmt.Println("🔍 Analyzing Gmail labels...")

	result, err := o.analyzer.AnalyzeLabels()
	if err != nil {
		return fmt.Errorf("analysis failed: %v", err)
	}

	if len(result.PeriodLabels) == 0 {
		fmt.Println("✅ No period-separated labels found. Your labels are already properly structured!")
		return nil
	}

	fmt.Printf("\n📊 Found %d period-separated labels with %d total messages\n", len(result.PeriodLabels), result.TotalMessages)

	// Debug: Show first few labels for troubleshooting
	fmt.Printf("\n🔍 Sample labels found:\n")
	count := 0
	for _, label := range result.PeriodLabels {
		if count < 5 {
			fmt.Printf("   - %s (ID: %s)\n", label.Name, label.Id)
			count++
		}
	}
	if len(result.PeriodLabels) > 5 {
		fmt.Printf("   ... and %d more\n", len(result.PeriodLabels)-5)
	}
	fmt.Println()

	// Check for conflicts
	conflicts := o.analyzer.CheckConflicts(result.Transformations)
	if len(conflicts) > 0 {
		fmt.Println("⚠️  CONFLICTS DETECTED:")
		for _, conflict := range conflicts {
			fmt.Printf("   - %s\n", conflict)
		}
		fmt.Println()
	}

	// Display transformations table
	o.displayTransformationsTable(result.Transformations)

	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("   - Fix specific label: gmail-label-fixer fix --label \"LabelName\"\n")
	fmt.Printf("   - Fix all labels: gmail-label-fixer fix --all\n")

	return nil
}

func (o *Operations) displayTransformationsTable(transformations map[string]*analyzer.LabelTransformation) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{"Current Label", "New Nested Structure", "Messages"}),
	)

	// Sort labels for consistent output
	var labels []string
	for label := range transformations {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	for _, label := range labels {
		transformation := transformations[label]

		table.Append([]string{
			transformation.OriginalLabel,
			transformation.NestedStructure,
			strconv.Itoa(transformation.MessageCount),
		})
	}

	table.Render()
}

func (o *Operations) FixLabel(labelName string) error {
	fmt.Printf("🔧 Fixing label: %s\n", labelName)

	// Find the specific label and all its children
	transformations, err := o.findLabelWithChildren(labelName)
	if err != nil {
		return err
	}

	if len(transformations) == 1 {
		// Single label
		transformation := transformations[0]
		fmt.Printf("   %s → %s\n", transformation.OriginalLabel, transformation.NestedStructure)
		return o.processTransformation(transformation)
	} else {
		// Parent label with children
		fmt.Printf("   Found %d labels (parent + %d children) to fix:\n", len(transformations), len(transformations)-1)
		for i, transformation := range transformations {
			fmt.Printf("   [%d/%d] %s → %s\n", i+1, len(transformations), transformation.OriginalLabel, transformation.NestedStructure)
		}

		// Process all transformations
		processed := 0
		for i, transformation := range transformations {
			fmt.Printf("\n[%d/%d] Processing: %s\n", i+1, len(transformations), transformation.OriginalLabel)

			if err := o.processTransformation(transformation); err != nil {
				fmt.Printf("❌ Failed: %v\n", err)
				continue
			}

			processed++
			fmt.Printf("✅ Success: %s → %s\n", transformation.OriginalLabel, transformation.NestedStructure)
		}

		fmt.Printf("\n🎉 Completed! Processed %d/%d labels successfully.\n", processed, len(transformations))
		return nil
	}
}

// findLabelWithChildren finds a label and all its children for hierarchical processing
func (o *Operations) findLabelWithChildren(labelName string) ([]*analyzer.LabelTransformation, error) {
	// Get all period-separated labels
	periodLabels, err := o.client.FindPeriodSeparatedLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to find labels: %v", err)
	}

	var matchingLabels []*gmailAPI.Label
	labelPrefix := labelName + "."

	// Find the target label and all its children
	for _, label := range periodLabels {
		if label.Name == labelName {
			// Exact match - this is our target label
			matchingLabels = append(matchingLabels, label)
		} else if strings.HasPrefix(label.Name, labelPrefix) {
			// This is a child label
			matchingLabels = append(matchingLabels, label)
		}
	}

	if len(matchingLabels) == 0 {
		return nil, fmt.Errorf("label '%s' not found or is not period-separated", labelName)
	}

	// Sort labels to process parents before children (shorter names first)
	sort.Slice(matchingLabels, func(i, j int) bool {
		return len(strings.Split(matchingLabels[i].Name, ".")) < len(strings.Split(matchingLabels[j].Name, "."))
	})

	var transformations []*analyzer.LabelTransformation

	// Create transformations for all matching labels
	for _, label := range matchingLabels {
		transformation := analyzer.ParseLabelHierarchy(label.Name)
		if transformation == nil {
			fmt.Printf("   ⚠️  Skipping invalid label format: %s\n", label.Name)
			continue // Skip invalid labels
		}

		transformation.OriginalID = label.Id

		// Get message count with proper error logging
		messageIDs, err := o.client.GetMessagesWithLabel(label.Id)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: Could not count messages for label %s: %v\n", label.Name, err)
			transformation.MessageCount = 0 // Continue anyway
		} else {
			transformation.MessageCount = len(messageIDs)
		}

		transformations = append(transformations, transformation)
	}

	return transformations, nil
}

// findSpecificLabel finds and analyzes a single label without verbose output
func (o *Operations) findSpecificLabel(labelName string) (*analyzer.LabelTransformation, error) {
	// Get all period-separated labels
	periodLabels, err := o.client.FindPeriodSeparatedLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to find labels: %v", err)
	}

	// Find the specific label
	var targetLabel *gmailAPI.Label
	for _, label := range periodLabels {
		if label.Name == labelName {
			targetLabel = label
			break
		}
	}

	if targetLabel == nil {
		return nil, fmt.Errorf("label '%s' not found or is not period-separated", labelName)
	}

	// Create transformation for this specific label
	transformation := analyzer.ParseLabelHierarchy(targetLabel.Name)
	if transformation == nil {
		return nil, fmt.Errorf("label '%s' is not period-separated", labelName)
	}

	transformation.OriginalID = targetLabel.Id

	// Get message count with proper error handling
	messageIDs, err := o.client.GetMessagesWithLabel(targetLabel.Id)
	if err != nil {
		fmt.Printf("   ⚠️  Warning: Could not count messages for label %s: %v\n", targetLabel.Name, err)
		transformation.MessageCount = 0 // Continue anyway
	} else {
		transformation.MessageCount = len(messageIDs)
	}

	return transformation, nil
}

func (o *Operations) FixAllLabels() error {
	fmt.Println("🔧 Fixing all period-separated labels...")

	result, err := o.analyzer.AnalyzeLabels()
	if err != nil {
		return fmt.Errorf("analysis failed: %v", err)
	}

	if len(result.Transformations) == 0 {
		fmt.Println("✅ No period-separated labels found!")
		return nil
	}

	// Process all transformations - Gmail will automatically create parent hierarchy when renaming
	processed := 0
	for _, transformation := range result.Transformations {
		fmt.Printf("\n[%d/%d] Processing: %s\n", processed+1, len(result.Transformations), transformation.OriginalLabel)

		if err := o.processTransformation(transformation); err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			continue
		}

		processed++
		fmt.Printf("✅ Success: %s → %s\n", transformation.OriginalLabel, transformation.NestedStructure)
	}

	fmt.Printf("\n🎉 Completed! Processed %d/%d labels successfully.\n", processed, len(result.Transformations))
	return nil
}

func (o *Operations) processTransformation(transformation *analyzer.LabelTransformation) error {
	// Check if target label name already exists
	if existingLabel, exists := o.client.LabelExists(transformation.NestedStructure); exists {
		return fmt.Errorf("target label '%s' already exists (ID: %s). Cannot rename to existing label", transformation.NestedStructure, existingLabel.Id)
	}

	// Simply rename the label - Gmail automatically preserves all message associations!
	fmt.Printf("   Renaming label: %s → %s\n", transformation.OriginalLabel, transformation.NestedStructure)

	var renamedLabel *gmailAPI.Label
	err := o.retryWithBackoff(func() error {
		var err error
		renamedLabel, err = o.client.RenameLabel(transformation.OriginalID, transformation.NestedStructure)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to rename label: %v", err)
	}

	o.withRateLimit()

	fmt.Printf("   ✅ Successfully renamed to: %s (ID: %s)\n", renamedLabel.Name, renamedLabel.Id)
	fmt.Printf("   📧 All %d messages automatically preserved\n", transformation.MessageCount)

	return nil
}
