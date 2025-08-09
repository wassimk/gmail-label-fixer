package analyzer

import (
	"fmt"
	"gmail-label-fixer/internal/gmail"
	"sort"

	gmailAPI "google.golang.org/api/gmail/v1"
)

type AnalysisResult struct {
	PeriodLabels     []*gmailAPI.Label
	Transformations  map[string]*LabelTransformation
	RequiredParents  []string
	TotalMessages    int
}

type Analyzer struct {
	client *gmail.Client
}

func NewAnalyzer(client *gmail.Client) *Analyzer {
	return &Analyzer{client: client}
}

func (a *Analyzer) AnalyzeLabels() (*AnalysisResult, error) {
	periodLabels, err := a.client.FindPeriodSeparatedLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to find period-separated labels: %v", err)
	}

	transformations := make(map[string]*LabelTransformation)
	totalMessages := 0

	for _, label := range periodLabels {
		transformation := ParseLabelHierarchy(label.Name)
		if transformation != nil {
			transformation.OriginalID = label.Id

			// Get message count for this label
			messageIDs, err := a.client.GetMessagesWithLabel(label.Id)
			if err != nil {
				return nil, fmt.Errorf("failed to get message count for label %s: %v", label.Name, err)
			}
			transformation.MessageCount = len(messageIDs)
			totalMessages += len(messageIDs)

			transformations[label.Name] = transformation
		}
	}

	requiredParents := GetAllRequiredParents(transformations)
	sort.Strings(requiredParents)

	return &AnalysisResult{
		PeriodLabels:    periodLabels,
		Transformations: transformations,
		RequiredParents: requiredParents,
		TotalMessages:   totalMessages,
	}, nil
}

func (a *Analyzer) CheckConflicts(transformations map[string]*LabelTransformation) []string {
	var conflicts []string
	
	// Check if any required parent names conflict with existing labels
	for _, transformation := range transformations {
		for _, parentName := range transformation.RequiredParents {
			if existingLabel, exists := a.client.LabelExists(parentName); exists {
				conflicts = append(conflicts, fmt.Sprintf("Parent label '%s' already exists (ID: %s)", parentName, existingLabel.Id))
			}
		}
		
		// Check if the target nested name already exists
		if existingLabel, exists := a.client.LabelExists(transformation.NestedStructure); exists {
			conflicts = append(conflicts, fmt.Sprintf("Target label '%s' already exists (ID: %s)", transformation.NestedStructure, existingLabel.Id))
		}
	}
	
	return conflicts
}