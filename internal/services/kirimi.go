package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type KirimiService struct {
	UserCode string
	DeviceID string
	Secret   string
	BaseURL  string
}

func NewKirimiService() *KirimiService {
	return &KirimiService{
		UserCode: os.Getenv("KIRIMI_USER_CODE"),
		DeviceID: os.Getenv("KIRIMI_DEVICE_ID"),
		Secret:   os.Getenv("KIRIMI_SECRET"),
		BaseURL:  "https://api.kirimi.id/v1",
	}
}

type KirimiSendRequest struct {
	UserCode          string `json:"user_code"`
	DeviceID          string `json:"device_id"`
	Receiver          string `json:"receiver"`
	Message           string `json:"message"`
	MediaURL          string `json:"media_url,omitempty"`
	FileName          string `json:"fileName,omitempty"`
	Secret            string `json:"secret"`
	EnableTypingEffect bool  `json:"enableTypingEffect"`
	TypingSpeedMs     int    `json:"typingSpeedMs,omitempty"`
	QuotedMessageID   string `json:"quotedMessageId,omitempty"`
}

// SendMessage mengirim pesan teks ke nomor WhatsApp via Kirimi API
func (s *KirimiService) SendMessage(target, message string) error {
	payload := KirimiSendRequest{
		UserCode:          s.UserCode,
		DeviceID:          s.DeviceID,
		Receiver:          target,
		Message:           message,
		Secret:            s.Secret,
		EnableTypingEffect: true,
		TypingSpeedMs:     350,
	}
	return s.doRequest("/send-message", payload)
}

// SendImage mengirim pesan dengan media (gambar) via Kirimi API
func (s *KirimiService) SendImage(target, message, imageUrl string) error {
	payload := KirimiSendRequest{
		UserCode:          s.UserCode,
		DeviceID:          s.DeviceID,
		Receiver:          target,
		Message:           message,
		MediaURL:          imageUrl,
		Secret:            s.Secret,
		EnableTypingEffect: true,
		TypingSpeedMs:     500,
	}
	return s.doRequest("/send-message", payload)
}

// SendLinkButton - Kirimi tidak punya template button, jadi kirim sebagai teks biasa dengan link
func (s *KirimiService) SendLinkButton(target, message, buttonText, url string) error {
	// Karena Kirimi tidak support template button seperti Fonnte,
	// kita gabungkan link ke dalam pesan teks
	fullMessage := fmt.Sprintf("%s\n\n🔗 %s: %s", message, buttonText, url)
	return s.SendMessage(target, fullMessage)
}

func (s *KirimiService) doRequest(endpoint string, payload interface{}) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kirimi marshal error: %v", err)
	}

	fmt.Printf("[Kirimi] Sending request to %s: %s\n", endpoint, string(jsonPayload))

	req, err := http.NewRequest("POST", s.BaseURL+endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("kirimi request error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[Kirimi] Response Status: %d, Body: %s\n", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kirimi error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
