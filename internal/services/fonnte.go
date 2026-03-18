package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type FonnteService struct {
	Token   string
	BaseURL string
}

func NewFonnteService() *FonnteService {
	return &FonnteService{
		Token:   os.Getenv("FONNTE_TOKEN"),
		BaseURL: "https://api.fonnte.com",
	}
}

type FonnteSendRequest struct {
	Target  string `json:"target"`
	Message string `json:"message"`
	Template string `json:"template,omitempty"`
}

func (s *FonnteService) SendMessage(target, message string) error {
	payload := FonnteSendRequest{
		Target:  target,
		Message: message,
	}
	return s.doRequest("/send", payload)
}

func (s *FonnteService) SendLinkButton(target, message, buttonText, url string) error {
	// Fonnte Template Format for Link Button
	// Format: [{"type": "url", "title": "View Item", "url": "https://fonnte.com"}]
	template := []map[string]string{
		{
			"type":  "url",
			"title": buttonText,
			"url":   url,
		},
	}
	templateData, _ := json.Marshal(template)

	payload := FonnteSendRequest{
		Target:   target,
		Message:  message,
		Template: string(templateData),
	}
	return s.doRequest("/send", payload)
}

func (s *FonnteService) doRequest(endpoint string, payload interface{}) error {
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.BaseURL+endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", s.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fonnte error: %s", string(body))
	}

	return nil
}
