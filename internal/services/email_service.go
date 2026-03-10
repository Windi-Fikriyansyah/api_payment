package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type EmailService struct {
	APIKey      string
	SenderEmail string
	SenderName  string
}

func NewEmailService() *EmailService {
	return &EmailService{
		APIKey:      os.Getenv("BREVO_API_KEY"),
		SenderEmail: os.Getenv("BREVO_SENDER_EMAIL"),
		SenderName:  os.Getenv("BREVO_SENDER_NAME"),
	}
}

type BrevoRecipient struct {
	Email string `json:"email"`
}

type BrevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoPayload struct {
	Sender      BrevoSender      `json:"sender"`
	To          []BrevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HtmlContent string           `json:"htmlContent"`
}

func (s *EmailService) SendPaymentSuccessEmail(toEmail string, projectName string, orderID string, amount float64) error {
	if s.APIKey == "" || toEmail == "" {
		return fmt.Errorf("email configuration or recipient is missing")
	}

	url := "https://api.brevo.com/v3/smtp/email"
	subject := fmt.Sprintf("Pembayaran Berhasil - %s", orderID)

	htmlContent := fmt.Sprintf(`
		<h3>Notifikasi Pembayaran Berhasil</h3>
		<p>Halo,</p>
		<p>Terdapat pembayaran berhasil pada project <b>%s</b>.</p>
		<ul>
			<li><b>Order ID:</b> %s</li>
			<li><b>Jumlah:</b> Rp %.2f</li>
			<li><b>Status:</b> Sukses</li>
		</ul>
		<p>Terima kasih.</p>
	`, projectName, orderID, amount)

	payload := BrevoPayload{
		Sender: BrevoSender{
			Name:  s.SenderName,
			Email: s.SenderEmail,
		},
		To: []BrevoRecipient{
			{Email: toEmail},
		},
		Subject:     subject,
		HtmlContent: htmlContent,
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("api-key", s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo api returned error: %s", resp.Status)
	}

	fmt.Printf("Email notification sent to %s for Order %s\n", toEmail, orderID)
	return nil
}
