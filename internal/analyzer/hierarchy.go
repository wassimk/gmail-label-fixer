package analyzer

import (
	"strings"
)

type LabelTransformation struct {
	OriginalLabel    string
	OriginalID       string
	HierarchyParts   []string
	NestedStructure  string
	MessageCount     int
	RequiredParents  []string
}

func ParseLabelHierarchy(labelName string) *LabelTransformation {
	parts := strings.Split(labelName, ".")
	if len(parts) <= 1 {
		return nil // Not a period-separated label
	}

	// Special handling for INBOX prefix - remove it entirely
	finalParts := parts
	if len(parts) > 1 && strings.ToUpper(parts[0]) == "INBOX" {
		finalParts = parts[1:] // Remove INBOX prefix
		
		// If after removing INBOX there's only one part left, it becomes a root label
		if len(finalParts) == 1 {
			transformation := &LabelTransformation{
				OriginalLabel:    labelName,
				HierarchyParts:   finalParts,
				NestedStructure:  finalParts[0], // Just the label name, no hierarchy
				RequiredParents:  []string{},    // No parents needed
			}
			return transformation
		}
	}

	transformation := &LabelTransformation{
		OriginalLabel:   labelName,
		HierarchyParts:  finalParts,
		NestedStructure: strings.Join(finalParts, "/"),
	}

	// Build required parent labels (excluding any INBOX prefix)
	for i := 1; i < len(finalParts); i++ {
		parentPath := strings.Join(finalParts[:i], "/")
		transformation.RequiredParents = append(transformation.RequiredParents, parentPath)
	}

	return transformation
}

func BuildHierarchyMap(labels []string) map[string]*LabelTransformation {
	transformations := make(map[string]*LabelTransformation)
	
	for _, labelName := range labels {
		if transformation := ParseLabelHierarchy(labelName); transformation != nil {
			transformations[labelName] = transformation
		}
	}
	
	return transformations
}

func GetAllRequiredParents(transformations map[string]*LabelTransformation) []string {
	parentsSet := make(map[string]bool)
	
	for _, transformation := range transformations {
		for _, parent := range transformation.RequiredParents {
			parentsSet[parent] = true
		}
	}
	
	var parents []string
	for parent := range parentsSet {
		parents = append(parents, parent)
	}
	
	return parents
}