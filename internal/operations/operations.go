package operations

import (
	"fmt"
	"gmail-label-fixer/internal/analyzer"
	"gmail-label-fixer/internal/gmail"
	"os"
	"sort"
	"strconv"

	"github.com/olekukonko/tablewriter"
)

type Operations struct {
	client   *gmail.Client
	analyzer *analyzer.Analyzer
}

func NewOperations(client *gmail.Client) *Operations {
	return &Operations{
		client:   client,
		analyzer: analyzer.NewAnalyzer(client),
	}
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

	fmt.Printf("\n📊 Found %d period-separated labels with %d total messages\n\n", len(result.PeriodLabels), result.TotalMessages)

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

	// Process all transformations
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
	// Step 1: Create required parent labels
	for _, parentName := range transformation.RequiredParents {
		if _, exists := o.client.LabelExists(parentName); !exists {
			fmt.Printf("   Creating parent label: %s\n", parentName)
			_, err := o.client.CreateLabel(parentName)
			if err != nil {
				return fmt.Errorf("failed to create parent label %s: %v", parentName, err)
			}
		}
	}

	// Step 2: Create the final nested label
	fmt.Printf("   Creating nested label: %s\n", transformation.NestedStructure)
	newLabel, err := o.client.CreateLabel(transformation.NestedStructure)
	if err != nil {
		return fmt.Errorf("failed to create nested label %s: %v", transformation.NestedStructure, err)
	}

	// Step 3: Move all messages from old label to new label
	messageIDs, err := o.client.GetMessagesWithLabel(transformation.OriginalID)
	if err != nil {
		return fmt.Errorf("failed to get messages for label %s: %v", transformation.OriginalLabel, err)
	}

	if len(messageIDs) > 0 {
		fmt.Printf("   Moving %d messages to new label...\n", len(messageIDs))
		for _, messageID := range messageIDs {
			err := o.client.ModifyMessageLabels(
				messageID,
				[]string{newLabel.Id},
				[]string{transformation.OriginalID},
			)
			if err != nil {
				return fmt.Errorf("failed to move message %s: %v", messageID, err)
			}
		}
	}

	// Step 4: Delete the original period-separated label
	fmt.Printf("   Deleting original label: %s\n", transformation.OriginalLabel)
	err = o.client.DeleteLabel(transformation.OriginalID)
	if err != nil {
		return fmt.Errorf("failed to delete original label %s: %v", transformation.OriginalLabel, err)
	}

	return nil
}