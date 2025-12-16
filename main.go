package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
	"golang.org/x/sync/errgroup"

	_ "golang.org/x/image/webp"
)

type ImageProcessor struct {
	s3Client          *s3.S3
	notifier          *Notifier
	destinationBucket string
}

type ImageSize struct {
	Name   string
	Width  int
	Height int
	Format string // "default", "square", "landscape", "portrait"
}

var supportedFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

var imageSizes = []ImageSize{
	{Name: "thumbnail", Width: 200, Height: 200, Format: "square"},
	{Name: "small", Width: 500, Height: 500},
	{Name: "medium", Width: 900, Height: 900},
	{Name: "large", Width: 1400, Height: 1400},
}

func NewImageProcessor() *ImageProcessor {
	sess := session.Must(session.NewSession())
	return &ImageProcessor{
		s3Client:          s3.New(sess),
		notifier:          NewNotifier(),
		destinationBucket: os.Getenv("DESTINATION_BUCKET"),
	}
}

func (ip *ImageProcessor) HandleS3Event(ctx context.Context, s3Event events.S3Event) error {
	for _, record := range s3Event.Records {
		bucketName := record.S3.Bucket.Name
		objectKey := record.S3.Object.Key

		if !ip.isValidImageFormat(objectKey) {
			log.Printf("Skipping non-image file: %s", objectKey)
			continue
		}

		if err := ip.processImage(ctx, bucketName, objectKey); err != nil {
			log.Printf("Error processing image %s: %v", objectKey, err)
			return err
		}
	}
	return nil
}

func (ip *ImageProcessor) isValidImageFormat(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return supportedFormats[ext]
}

func (ip *ImageProcessor) resizeImage(img image.Image, size ImageSize) image.Image {
	format := size.Format
	if format == "" {
		format = "default"
	}

	switch format {
	case "square":
		return imaging.Fill(img, size.Width, size.Height, imaging.Center, imaging.Linear)
	case "landscape":
		return imaging.Resize(img, size.Width, 0, imaging.Linear)
	case "portrait":
		return imaging.Resize(img, 0, size.Height, imaging.Linear)
	case "default":
		fallthrough
	default:
		return imaging.Fit(img, size.Width, size.Height, imaging.Linear)
	}
}

func (ip *ImageProcessor) processImage(ctx context.Context, bucket, key string) error {
	// Safety check: prevent infinite loops by ensuring source and destination buckets are different
	if bucket == ip.destinationBucket {
		return fmt.Errorf("source bucket (%s) and destination bucket (%s) cannot be the same to prevent infinite loops", bucket, ip.destinationBucket)
	}

	originalImage, format, metadata, err := ip.downloadImage(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}

	isReplacement := false
	if metadata.ExistingFile != "" {
		isReplacement = true
	}

	dir := filepath.Dir(key)
	baseName := strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))
	originalExt := filepath.Ext(key)

	// Process images in parallel
	g, gCtx := errgroup.WithContext(ctx)
	semaphore := make(chan struct{}, runtime.NumCPU())

	for _, size := range imageSizes {
		size := size            // capture loop variable
		semaphore <- struct{}{} // acquire semaphore

		g.Go(func() error {
			defer func() { <-semaphore }() // release semaphore

			resizedImage := ip.resizeImage(originalImage, size)
			newKey := filepath.Join(dir, fmt.Sprintf("%s_%s%s", baseName, size.Name, originalExt))

			if err := ip.uploadImage(gCtx, ip.destinationBucket, newKey, resizedImage, format); err != nil {
				return fmt.Errorf("failed to upload resized image %s: %w", newKey, err)
			}

			log.Printf("Successfully created %s", newKey)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Send webhook notification after all sizes are processed
	if ip.notifier.IsConfigured() {
		if err := ip.notifier.SendImageProcessedNotification(bucket, key, ip.destinationBucket, imageSizes, metadata.BrandID, metadata.EntityType, metadata.EntityID, metadata.RequestedBy, isReplacement); err != nil {
			log.Printf("Failed to send webhook notification: %v", err)
			// Don't return error - image processing was successful
		} else {
			log.Printf("Webhook notification sent successfully for %s", key)
		}
	}

	return nil
}

type ImageMetadata struct {
	BrandID      string
	EntityType   string
	EntityID     string
	RequestedBy  string
	ExistingFile string
}

func (ip *ImageProcessor) downloadImage(ctx context.Context, bucket, key string) (image.Image, string, *ImageMetadata, error) {
	result, err := ip.s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", nil, err
	}
	defer result.Body.Close()

	log.Printf("METADATA: %+v", result.Metadata)
	// Extract metadata from S3 object
	metadata := &ImageMetadata{
		BrandID:      getMetadataValue(result.Metadata, "Brandid"),
		EntityType:   getMetadataValue(result.Metadata, "Entitytype"),
		EntityID:     getMetadataValue(result.Metadata, "Entityid"),
		RequestedBy:  getMetadataValue(result.Metadata, "Requestedby"),
		ExistingFile: getMetadataValue(result.Metadata, "Existingfile"),
	}

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", nil, err
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, format, metadata, nil
}

func getMetadataValue(metadata map[string]*string, key string) string {
	if value, exists := metadata[key]; exists && value != nil {
		return *value
	}
	return ""
}

func (ip *ImageProcessor) uploadImage(ctx context.Context, bucket, key string, img image.Image, format string) error {
	var buf bytes.Buffer
	var contentType string

	switch format {
	case "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return err
		}
		contentType = "image/jpeg"
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		contentType = "image/png"
	case "webp":
		// For WebP output, convert to JPEG with high quality
		// This is a reasonable fallback since WebP encoding requires CGO
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return err
		}
		contentType = "image/jpeg"
	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return err
		}
		contentType = "image/gif"
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	_, err := ip.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String(contentType),
	})

	return err
}

func main() {
	processor := NewImageProcessor()
	lambda.Start(processor.HandleS3Event)
}
