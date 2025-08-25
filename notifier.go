package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ImageSizeInfo represents information about a processed image size
type ImageSizeInfo struct {
	Name   string `json:"name"`   // thumbnail, small, medium, large
	URL    string `json:"url"`    // full S3 URL
	Key    string `json:"key"`    // S3 key
	Width  int    `json:"width"`  // target width
	Height int    `json:"height"` // target height
}

// WebhookPayload represents the notification payload for image processing
type WebhookPayload struct {
	OriginalFile  string          `json:"original_file"`  // original file key
	OriginalURL   string          `json:"original_url"`   // original file URL
	Bucket        string          `json:"bucket"`         // S3 bucket name
	ProcessedAt   string          `json:"processed_at"`   // timestamp
	Environment   string          `json:"environment"`    // deployment environment
	TotalSizes    int             `json:"total_sizes"`    // number of sizes created
	ImageSizes    []ImageSizeInfo `json:"image_sizes"`    // array of processed sizes
	EventType     string          `json:"event_type"`     // always "image_processed"
}

// Notifier handles webhook notifications
type Notifier struct {
	webhookURL    string
	webhookSecret string
	region        string
	client        *http.Client
}

// NewNotifier creates a new notifier instance
func NewNotifier() *Notifier {
	return &Notifier{
		webhookURL:    os.Getenv("WEBHOOK_URL"),
		webhookSecret: os.Getenv("WEBHOOK_SECRET"),
		region:        os.Getenv("AWS_REGION"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsConfigured checks if webhook is properly configured
func (n *Notifier) IsConfigured() bool {
	return n.webhookURL != ""
}

// SendImageProcessedNotification sends notification about processed image
func (n *Notifier) SendImageProcessedNotification(bucket, originalKey string, processedSizes []ImageSize) error {
	if !n.IsConfigured() {
		log.Printf("Webhook not configured, skipping notification")
		return nil
	}

	// Build image sizes info
	imageSizes := make([]ImageSizeInfo, 0, len(processedSizes))
	
	for _, size := range processedSizes {
		// Generate the processed file key (following same pattern as main.go)
		dir := getFileDir(originalKey)
		baseName := getFileBaseName(originalKey)
		ext := getFileExt(originalKey)
		
		processedKey := fmt.Sprintf("%s/%s_%s%s", dir, baseName, size.Name, ext)
		if dir == "" {
			processedKey = fmt.Sprintf("%s_%s%s", baseName, size.Name, ext)
		}
		
		imageSize := ImageSizeInfo{
			Name:   size.Name,
			URL:    n.generateFileURL(bucket, processedKey),
			Key:    processedKey,
			Width:  size.Width,
			Height: size.Height,
		}
		imageSizes = append(imageSizes, imageSize)
	}

	payload := &WebhookPayload{
		OriginalFile: originalKey,
		OriginalURL:  n.generateFileURL(bucket, originalKey),
		Bucket:       bucket,
		ProcessedAt:  time.Now().UTC().Format(time.RFC3339),
		Environment:  os.Getenv("ENVIRONMENT"),
		TotalSizes:   len(imageSizes),
		ImageSizes:   imageSizes,
		EventType:    "image_processed",
	}

	return n.sendWebhook(payload)
}

// sendWebhook sends the webhook with retry logic
func (n *Notifier) sendWebhook(payload *WebhookPayload) error {
	const maxRetries = 3
	const baseDelay = time.Second

	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			log.Printf("Retrying webhook notification (attempt %d/%d) after %v", attempt+1, maxRetries, delay)
			time.Sleep(delay)
		}

		err := n.sendSingleWebhook(payload)
		if err == nil {
			log.Printf("Webhook notification sent successfully to: %s", n.webhookURL)
			return nil
		}

		lastErr = err
		log.Printf("Webhook notification failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
	}

	return fmt.Errorf("webhook notification failed after %d attempts: %w", maxRetries, lastErr)
}

// sendSingleWebhook performs a single webhook attempt
func (n *Notifier) sendSingleWebhook(payload *WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	signature := n.calculateSignature(jsonData)

	req, err := http.NewRequest("POST", n.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "endless-image-processor-lambda")
	req.Header.Set("X-EC-Signature", signature)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

// calculateSignature generates HMAC-SHA256 signature for webhook validation
func (n *Notifier) calculateSignature(jsonData []byte) string {
	if n.webhookSecret == "" {
		return ""
	}
	
	mac := hmac.New(sha256.New, []byte(n.webhookSecret))
	mac.Write(jsonData)
	signature := hex.EncodeToString(mac.Sum(nil))
	
	return "sha256=" + signature
}

// generateFileURL creates an S3 file URL
func (n *Notifier) generateFileURL(bucket, key string) string {
	region := n.region
	if region == "" {
		region = "us-east-1" // default fallback
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
}

// Helper functions to extract file path components
func getFileDir(key string) string {
	lastSlash := -1
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash == -1 {
		return ""
	}
	return key[:lastSlash]
}

func getFileBaseName(key string) string {
	// Get filename without directory
	lastSlash := -1
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			lastSlash = i
			break
		}
	}
	
	filename := key
	if lastSlash != -1 {
		filename = key[lastSlash+1:]
	}
	
	// Remove extension
	lastDot := -1
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			lastDot = i
			break
		}
	}
	
	if lastDot == -1 {
		return filename
	}
	return filename[:lastDot]
}

func getFileExt(key string) string {
	lastDot := -1
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			lastDot = i
			break
		}
		if key[i] == '/' {
			break
		}
	}
	
	if lastDot == -1 {
		return ""
	}
	return key[lastDot:]
}