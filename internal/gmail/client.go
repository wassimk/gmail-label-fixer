package gmail

import (
	"fmt"
	"strings"

	"google.golang.org/api/gmail/v1"
)

type Client struct {
	service *gmail.Service
	userID  string
}

func NewClient(service *gmail.Service) *Client {
	return &Client{
		service: service,
		userID:  "me",
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
		Name:                    name,
		MessageListVisibility:   "show",
		LabelListVisibility:     "labelShow",
	}

	call := c.service.Users.Labels.Create(c.userID, label)
	createdLabel, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create label %s: %v", name, err)
	}
	return createdLabel, nil
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
	query := fmt.Sprintf("label:%s", labelID)
	call := c.service.Users.Messages.List(c.userID).Q(query)
	
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

func (c *Client) FindPeriodSeparatedLabels() ([]*gmail.Label, error) {
	labels, err := c.GetAllLabels()
	if err != nil {
		return nil, err
	}
	
	var periodLabels []*gmail.Label
	for _, label := range labels {
		if label.Type == "user" && strings.Contains(label.Name, ".") {
			periodLabels = append(periodLabels, label)
		}
	}
	
	return periodLabels, nil
}