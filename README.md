# Endless Image Processor

A serverless AWS Lambda function that automatically processes images uploaded to S3 buckets by creating multiple resized versions with webhook notifications.

## Features

- **Automatic image processing** triggered by S3 events
- **Multiple sizes**: Creates 4 different sizes - thumbnail (150x150), small (400x400), medium (800x800), large (1200x1200)
- **Format support**: JPEG, PNG, WebP, and GIF formats
- **Smart resizing**: Preserves aspect ratio and prevents upscaling
- **High quality**: Uses Lanczos algorithm for optimal image quality
- **Webhook notifications**: Sends processing results to configurable endpoints
- **Secure webhooks**: HMAC-SHA256 signature validation
- **Built with Go** for optimal performance and minimal cold starts

## Project Structure

```
├── main.go              # Main Lambda function
├── notifier.go          # Webhook notification system
├── go.mod              # Go dependencies
├── go.sum              # Go dependency checksums
├── template-env.yaml   # SAM template for deployment
├── samconfig.toml      # SAM configuration
├── Makefile            # Build and deployment commands
├── .github/workflows/  # GitHub Actions deployment
├── .gitignore          # Git ignore patterns
├── DEPLOYMENT.md       # Detailed deployment guide
└── README.md          # This file
```

## Requirements

- Go 1.21+
- AWS account with proper permissions
- GitHub repository with Actions enabled

## How It Works

1. Image uploaded to S3 bucket
2. S3 event triggers Lambda function
3. Lambda validates supported format (.jpg, .jpeg, .png, .webp, .gif)  
   Note: WebP input supported, WebP output converted to high-quality JPEG
4. Generates 4 resized versions using imaging library
5. Saves all versions to same directory with suffixes
6. Optionally sends webhook notification with all URLs

## Example

**Original file**: `photos/vacation/beach.jpg`

**Generated files**:
- `photos/vacation/beach_thumbnail.jpg` (150x150)
- `photos/vacation/beach_small.jpg` (400x400) 
- `photos/vacation/beach_medium.jpg` (800x800)
- `photos/vacation/beach_large.jpg` (1200x1200)

## Deployment

This project uses **GitHub Actions for manual deployment**. No automatic deployments on push.

### Quick Setup

1. **Configure GitHub Secrets**:
   ```
   # AWS Credentials
   AWS_ACCESS_KEY_ID
   AWS_SECRET_ACCESS_KEY
   
   # S3 Buckets by Environment
   DEV_BUCKET_NAME
   STAGING_BUCKET_NAME  
   PROD_BUCKET_NAME
   
   # Webhook Configuration by Environment (Optional)
   DEV_WEBHOOK_URL
   DEV_WEBHOOK_SECRET
   STAGING_WEBHOOK_URL
   STAGING_WEBHOOK_SECRET
   PROD_WEBHOOK_URL
   PROD_WEBHOOK_SECRET
   ```

2. **Deploy**:
   - Go to Actions tab in GitHub
   - Run "Deploy Image Processor Lambda" workflow
   - Select only:
     - **Environment**: dev/staging/prod
     - **Branch**: branch to deploy from
   - All other configuration (buckets, webhooks) is automatic per environment

### Environment Configuration

| Environment | Memory | Timeout | Use Case |
|-------------|--------|---------|----------|
| dev         | 256MB  | 180s    | Development & testing |
| staging     | 512MB  | 300s    | Pre-production testing |
| prod        | 1024MB | 300s    | Production workloads |

## Webhook Notifications

When configured, the Lambda sends webhook notifications after processing:

```json
{
  "event_type": "image_processed",
  "original_file": "photos/vacation/beach.jpg",
  "original_url": "https://my-prod-bucket.s3.us-east-1.amazonaws.com/photos/vacation/beach.jpg",
  "bucket": "my-prod-bucket",
  "processed_at": "2024-01-15T10:30:00Z",
  "environment": "prod",
  "total_sizes": 4,
  "image_sizes": [
    {
      "name": "thumbnail",
      "url": "https://my-prod-bucket.s3.us-east-1.amazonaws.com/photos/vacation/beach_thumbnail.jpg",
      "key": "photos/vacation/beach_thumbnail.jpg",
      "width": 150,
      "height": 150
    },
    {
      "name": "small", 
      "url": "https://my-prod-bucket.s3.us-east-1.amazonaws.com/photos/vacation/beach_small.jpg",
      "key": "photos/vacation/beach_small.jpg",
      "width": 400,
      "height": 400
    }
    // ... medium, large
  ]
}
```

## Security

- **HMAC-SHA256 signatures** for webhook validation
- **IAM roles** with minimal required permissions (S3 read/write on specified bucket)
- **Uses existing buckets** - connects to pre-configured S3 buckets per environment
- **Environment isolation** - separate Lambda functions per environment

## Cost Optimization

- **Pay-per-use** - only costs when processing images
- **Memory optimized** by environment (256MB dev, 1024MB prod)
- **Go runtime** for fast cold starts and minimal memory usage
- **Conservative timeouts** prevent runaway costs

Estimated monthly cost: $0.20 (dev) to $15 (high-volume prod)

## Local Development

```bash
# Install dependencies
go mod tidy

# Build binary
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o main .

# Test compilation
go build .
```

## Documentation

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed deployment instructions and troubleshooting.