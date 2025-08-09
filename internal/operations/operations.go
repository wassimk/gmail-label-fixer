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
			RateLimitDelay: 200, // Default 200ms delay
			MaxRetries:     3,   // Default 3 retries
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
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			delay := baseDelay + jitter
			
			if delay > 32*time.Second {
				delay = 32*time.Second // Cap at 32 seconds
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
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout:     // 504
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

	// Display required parent labels
	if len(result.RequiredParents) > 0 {
		fmt.Printf("\n📁 Required parent labels to be created (%d):\n", len(result.RequiredParents))
		for _, parent := range result.RequiredParents {
			fmt.Printf("   - %s\n", parent)
		}
	}

	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("   - Fix specific label: gmail-label-fixer fix --label \"LabelName\"\n")
	fmt.Printf("   - Fix all labels: gmail-label-fixer fix --all\n")

	return nil
}

func (o *Operations) displayTransformationsTable(transformations map[string]*analyzer.LabelTransformation) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{"Current Label", "New Nested Structure", "Messages", "Required Parents"}),
	)

	// Sort labels for consistent output
	var labels []string
	for label := range transformations {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	for _, label := range labels {
		transformation := transformations[label]
		parentsStr := ""
		if len(transformation.RequiredParents) > 0 {
			parentsStr = fmt.Sprintf("%d parents", len(transformation.RequiredParents))
		}

		table.Append([]string{
			transformation.OriginalLabel,
			transformation.NestedStructure,
			strconv.Itoa(transformation.MessageCount),
			parentsStr,
		})
	}

	table.Render()
}

func (o *Operations) FixLabel(labelName string) error {
	fmt.Printf("🔧 Fixing label: %s\n", labelName)

	result, err := o.analyzer.AnalyzeLabels()
	if err != nil {
		return fmt.Errorf("analysis failed: %v", err)
	}

	transformation, exists := result.Transformations[labelName]
	if !exists {
		return fmt.Errorf("label '%s' not found or is not period-separated", labelName)
	}

	return o.processTransformation(transformation)
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

	// Step 1: Create all required parent labels first (avoids duplicates)
	if len(result.RequiredParents) > 0 {
		fmt.Printf("\n📁 Creating %d required parent labels...\n", len(result.RequiredParents))
		for i, parentName := range result.RequiredParents {
			if _, exists := o.client.LabelExists(parentName); !exists {
				fmt.Printf("   [%d/%d] Creating parent: %s\n", i+1, len(result.RequiredParents), parentName)
				
				err := o.retryWithBackoff(func() error {
					_, err := o.client.CreateLabel(parentName)
					return err
				})
				
				if err != nil {
					fmt.Printf("   ⚠️  Warning: Failed to create parent %s: %v\n", parentName, err)
				} else {
					o.withRateLimit() // Apply rate limit after successful operation
				}
			} else {
				fmt.Printf("   [%d/%d] Skipping existing parent: %s\n", i+1, len(result.RequiredParents), parentName)
			}
		}
	}

	// Step 2: Process all transformations
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
	// Step 1: Create required parent labels (only for selective fixes)
	for _, parentName := range transformation.RequiredParents {
		if _, exists := o.client.LabelExists(parentName); !exists {
			fmt.Printf("   Creating parent label: %s\n", parentName)
			err := o.retryWithBackoff(func() error {
				_, err := o.client.CreateLabel(parentName)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to create parent label %s: %v", parentName, err)
			}
			o.withRateLimit()
		}
	}

	// Step 2: Create the final nested label (check if it already exists)
	var newLabel *gmailAPI.Label
	if existingLabel, exists := o.client.LabelExists(transformation.NestedStructure); exists {
		fmt.Printf("   Using existing nested label: %s\n", transformation.NestedStructure)
		newLabel = existingLabel
	} else {
		fmt.Printf("   Creating nested label: %s\n", transformation.NestedStructure)
		err := o.retryWithBackoff(func() error {
			var err error
			newLabel, err = o.client.CreateLabel(transformation.NestedStructure)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to create nested label %s: %v", transformation.NestedStructure, err)
		}
		o.withRateLimit()
	}

	// Step 3: Move all messages from old label to new label (with batching)
	messageIDs, err := o.client.GetMessagesWithLabel(transformation.OriginalID)
	if err != nil {
		return fmt.Errorf("failed to get messages for label %s: %v", transformation.OriginalLabel, err)
	}

	if len(messageIDs) > 0 {
		fmt.Printf("   Moving %d messages to new label...\n", len(messageIDs))
		err := o.processBatchMessages(messageIDs, newLabel.Id, transformation.OriginalID)
		if err != nil {
			return fmt.Errorf("failed to move messages: %v", err)
		}
	}

	// Step 4: Delete the original period-separated label
	fmt.Printf("   Deleting original label: %s\n", transformation.OriginalLabel)
	err = o.retryWithBackoff(func() error {
		return o.client.DeleteLabel(transformation.OriginalID)
	})
	if err != nil {
		return fmt.Errorf("failed to delete original label %s: %v", transformation.OriginalLabel, err)
	}
	o.withRateLimit()

	return nil
}

// processBatchMessages handles batch processing of message label modifications
func (o *Operations) processBatchMessages(messageIDs []string, newLabelID, oldLabelID string) error {
	const batchSize = 25 // Process 25 messages at a time
	
	for i := 0; i < len(messageIDs); i += batchSize {
		end := i + batchSize
		if end > len(messageIDs) {
			end = len(messageIDs)
		}
		
		batch := messageIDs[i:end]
		fmt.Printf("     Processing batch %d-%d of %d messages...\n", i+1, end, len(messageIDs))
		
		// Process each message in the batch with rate limiting
		for j, messageID := range batch {
			err := o.retryWithBackoff(func() error {
				return o.client.ModifyMessageLabels(
					messageID,
					[]string{newLabelID},
					[]string{oldLabelID},
				)
			})
			
			if err != nil {
				fmt.Printf("     ⚠️  Warning: Failed to move message %d in batch: %v\n", j+1, err)
				continue // Continue with other messages in batch
			}
			
			// Apply rate limiting after each successful message
			o.withRateLimit()
		}
	}
	
	return nil
}