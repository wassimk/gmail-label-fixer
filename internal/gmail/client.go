package gmail

import (
	"fmt"
	"strings"

	"google.golang.org/api/gmail/v1"
)

const (
	userID = "me" // Gmail API user identifier for authenticated user
)

// System labels that should be skipped during label fixing
var skipLabels = map[string]bool{
	"INBOX.Trash":         true,
	"INBOX.Sent":          true,
	"INBOX.Sent Messages": true,
	"Inbox.Trash":         true,
	"Inbox.Sent":          true,
	"Inbox.Sent Messages": true,
}

// shouldSkipLabel checks if a label should be skipped during processing
func shouldSkipLabel(labelName string) bool {
	return skipLabels[labelName]
}

type Client struct {
	service *gmail.Service
	userID  string
}

func NewClient(service *gmail.Service) *Client {
	return &Client{
		service: service,
		userID:  userID,
	}
}

func (c *Client) GetAllLabels() ([]*gmail.Label, error) {
	call := c.service.Users.Labels.List(c.userID)
	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve labels: %v", err)
	}
	return response.Labels, nil
}

func (c *Client) CreateLabel(name string) (*gmail.Label, error) {
	label := &gmail.Label{
		Name:                  name,
		MessageListVisibility: "show",
		LabelListVisibility:   "labelShow",
	}

	call := c.service.Users.Labels.Create(c.userID, label)
	createdLabel, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create label %s: %v", name, err)
	}
	return createdLabel, nil
}

func (c *Client) RenameLabel(labelID, newName string) (*gmail.Label, error) {
	// Create the label patch with just the name change
	labelPatch := &gmail.Label{
		Name: newName,
	}

	call := c.service.Users.Labels.Patch(c.userID, labelID, labelPatch)
	updatedLabel, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to rename label %s to %s: %v", labelID, newName, err)
	}
	return updatedLabel, nil
}

func (c *Client) DeleteLabel(labelID string) error {
	call := c.service.Users.Labels.Delete(c.userID, labelID)
	err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to delete label %s: %v", labelID, err)
	}
	return nil
}

func (c *Client) GetMessagesWithLabel(labelID string) ([]string, error) {
	// Use labelId parameter instead of search query for more reliable results
	call := c.service.Users.Messages.List(c.userID).LabelIds(labelID)

	var messageIDs []string

	for {
		response, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get messages with label %s: %v", labelID, err)
		}

		for _, message := range response.Messages {
			messageIDs = append(messageIDs, message.Id)
		}

		if response.NextPageToken == "" {
			break
		}
		call.PageToken(response.NextPageToken)
	}

	return messageIDs, nil
}

func (c *Client) ModifyMessageLabels(messageID string, addLabelIDs, removeLabelIDs []string) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		AddLabelIds:    addLabelIDs,
		RemoveLabelIds: removeLabelIDs,
	}

	call := c.service.Users.Messages.Modify(c.userID, messageID, modifyRequest)
	_, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to modify message %s labels: %v", messageID, err)
	}
	return nil
}

func (c *Client) LabelExists(labelName string) (*gmail.Label, bool) {
	labels, err := c.GetAllLabels()
	if err != nil {
		return nil, false
	}

	for _, label := range labels {
		if label.Name == labelName {
			return label, true
		}
	}
	return nil, false
}

type LabelAnalysis struct {
	ProcessableLabels []*gmail.Label
	SkippedLabels     []*gmail.Label
}

func (c *Client) FindPeriodSeparatedLabels() ([]*gmail.Label, error) {
	analysis, err := c.FindPeriodSeparatedLabelsWithAnalysis()
	if err != nil {
		return nil, err
	}
	return analysis.ProcessableLabels, nil
}

func (c *Client) FindPeriodSeparatedLabelsWithAnalysis() (*LabelAnalysis, error) {
	labels, err := c.GetAllLabels()
	if err != nil {
		return nil, err
	}

	var processableLabels []*gmail.Label
	var skippedLabels []*gmail.Label

	for _, label := range labels {
		if label.Type == "user" && strings.Contains(label.Name, ".") {
			// Skip system labels that should not be processed
			if shouldSkipLabel(label.Name) {
				skippedLabels = append(skippedLabels, label)
				continue
			}
			processableLabels = append(processableLabels, label)
		}
	}

	return &LabelAnalysis{
		ProcessableLabels: processableLabels,
		SkippedLabels:     skippedLabels,
	}, nil
}
