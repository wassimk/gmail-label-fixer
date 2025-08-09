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

	transformation := &LabelTransformation{
		OriginalLabel:   labelName,
		HierarchyParts:  parts,
		NestedStructure: strings.Join(parts, "/"),
	}

	// Build required parent labels
	for i := 1; i < len(parts); i++ {
		parentPath := strings.Join(parts[:i], "/")
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