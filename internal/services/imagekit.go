package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type ImageKitConfig struct {
	PublicKey   string
	PrivateKey  string
	UrlEndpoint string
}

type ImageKitService struct {
	Config ImageKitConfig
}

func NewImageKitService(config ImageKitConfig) *ImageKitService {
	return &ImageKitService{Config: config}
}

func (s *ImageKitService) UploadQRIS(qrString string, fileName string) (string, string, error) {
	// We use an external QR generator to get an image URL
	qrUrl := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=%s", qrString)

	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	_ = writer.WriteField("file", qrUrl)
	_ = writer.WriteField("fileName", string(fileName))
	_ = writer.WriteField("folder", "/qris")
	writer.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", "https://upload.imagekit.io/api/v1/files/upload", &b)
	if err != nil {
		return "", "", err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(s.Config.PrivateKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("ImageKit upload failed: %s", string(respBody))
	}

	var result struct {
		URL    string `json:"url"`
		FileID string `json:"fileId"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", err
	}

	return result.URL, result.FileID, nil
}

func (s *ImageKitService) DeleteFile(fileID string) error {
	if fileID == "" {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://api.imagekit.io/v1/files/%s", fileID), nil)
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(s.Config.PrivateKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ImageKit delete failed: %s", string(respBody))
	}

	return nil
}
