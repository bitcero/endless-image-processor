# Endless Image Processor - Deployment Guide

## GitHub Actions Manual Deployment

This project uses GitHub Actions for manual deployment. It does **NOT** deploy automatically on push.

### GitHub Secrets Configuration

The following secrets must be configured in GitHub:

#### Required Secrets
- `AWS_ACCESS_KEY_ID`: AWS Access Key for deployment
- `AWS_SECRET_ACCESS_KEY`: AWS Secret Access Key for deployment
- `DEPLOYMENT_BUCKET_NAME`: S3 bucket for SAM deployment artifacts (shared across all environments)

#### Environment-Specific Secrets
- `DEV_BUCKET_NAME`: S3 bucket name for dev environment
- `STAGING_BUCKET_NAME`: S3 bucket name for staging environment  
- `PROD_BUCKET_NAME`: S3 bucket name for prod environment

#### Optional Webhook Secrets
- `DEV_WEBHOOK_URL` / `DEV_WEBHOOK_SECRET/`: Dev webhook configuration
- `STAGING_WEBHOOK_URL` / `STAGING_WEBHOOK_SECRET`: Staging webhook configuration
- `PROD_WEBHOOK_URL` / `PROD_WEBHOOK_SECRET`: Production webhook configuration

#### GitHub Configuration
1. Go to your repository on GitHub
2. Settings → Secrets and variables → Actions  
3. Click "New repository secret"
4. Add each secret with its corresponding value

### How to Deploy

1. Go to your repository on GitHub
2. Click on the "Actions" tab
3. Select the workflow "Deploy Image Processor Lambda"
4. Click "Run workflow"
5. Select deployment parameters:
   - **Environment**: `dev`, `staging`, or `prod`
   - **Branch**: branch you want to deploy (e.g., `main`, `develop`, `feature/xyz`)
6. Click "Run workflow"

**Note**: All environment-specific configuration (bucket names, webhook URLs) is automatically loaded from GitHub Secrets based on the selected environment.

### Environment Configuration

#### Development (dev)
- **Function**: `endless-image-processor-dev`
- **Memory**: 256MB
- **Timeout**: 180s

#### Staging
- **Function**: `endless-image-processor-staging`
- **Memory**: 512MB
- **Timeout**: 300s

#### Production (prod)
- **Function**: `endless-image-processor-prod`
- **Memory**: 1024MB
- **Timeout**: 300s

**Note**: Bucket names are specified during deployment and must be existing S3 buckets.

## AWS Resources Created

For each environment, the following resources are created:
- 1 Lambda Function (Go runtime) with webhook notification capability
- S3 Event trigger configured on existing bucket
- Required IAM roles and policies for bucket access

## Estimated Costs

### Lambda
- **Dev**: ~$0.20/month (low usage)
- **Staging**: ~$2-5/month (moderate testing)
- **Prod**: ~$5-15/month (depending on volume)

### S3
- **Storage**: $0.023/GB/month
- **Requests**: $0.0004/1000 PUT requests

### Important Considerations

⚠️ **IMPORTANT**: The lambda connects to existing S3 buckets. Make sure the bucket exists and you have proper permissions.

⚠️ **COSTS**: Each environment maintains its own resources. Delete unused environments to avoid unnecessary costs.

⚠️ **PERMISSIONS**: The lambda has full read/write permissions on its corresponding bucket.

## Webhook Notifications

The lambda can optionally send webhook notifications after processing images.

### Webhook Payload Example
```json
{
  "event_type": "image_processed",
  "original_file": "photos/vacation/beach.jpg",
  "original_url": "https://bucket.s3.region.amazonaws.com/photos/vacation/beach.jpg",
  "bucket": "my-images-bucket",
  "processed_at": "2024-01-15T10:30:00Z",
  "environment": "prod",
  "total_sizes": 4,
  "image_sizes": [
    {
      "name": "thumbnail",
      "url": "https://bucket.s3.region.amazonaws.com/photos/vacation/beach_thumbnail.jpg",
      "key": "photos/vacation/beach_thumbnail.jpg",
      "width": 150,
      "height": 150
    },
    {
      "name": "small",
      "url": "https://bucket.s3.region.amazonaws.com/photos/vacation/beach_small.jpg", 
      "key": "photos/vacation/beach_small.jpg",
      "width": 400,
      "height": 400
    },
    {
      "name": "medium",
      "url": "https://bucket.s3.region.amazonaws.com/photos/vacation/beach_medium.jpg",
      "key": "photos/vacation/beach_medium.jpg", 
      "width": 800,
      "height": 800
    },
    {
      "name": "large",
      "url": "https://bucket.s3.region.amazonaws.com/photos/vacation/beach_large.jpg",
      "key": "photos/vacation/beach_large.jpg",
      "width": 1200,
      "height": 1200
    }
  ]
}
```

### Webhook Security
- Requests include `X-EC-Signature` header with HMAC-SHA256 signature
- Signature format: `sha256=<hex_signature>`
- Uses webhook secret for signature validation
- Retry logic: 3 attempts with exponential backoff
- 30-second timeout per request

## Manual S3 Event Configuration

After deploying the Lambda function, you need to manually configure S3 event notifications **once per environment**:

### Steps to Configure S3 Events

1. **Go to AWS S3 Console**
2. **Select your bucket** (e.g., DEV_BUCKET_NAME for dev environment)
3. **Go to Properties tab** 
4. **Scroll to Event notifications section**
5. **Click "Create event notification"**
6. **Configure the event:**
   - **Name**: `image-processor-trigger`
   - **Event types**: Check `All object create events`
   - **Prefix**: (leave empty to process all files)
   - **Suffix**: Add multiple suffixes: `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`
   - **Destination**: Select `Lambda function`
   - **Lambda function**: Choose `endless-image-processor-{environment}`

7. **Save the configuration**

**Note**: This configuration is permanent and only needs to be done once per environment. Future Lambda deployments will not affect this S3 event configuration.

## Troubleshooting

### Error: Bucket does not exist
- Make sure the bucket name you specified exists
- Verify you have access permissions to the bucket

### Error: Stack already exists  
- A stack with that name already exists
- Go to CloudFormation in AWS Console and delete the previous stack

### Error: Access Denied
- Verify that AWS secrets are configured correctly
- Verify that the AWS account has permissions to create resources

## Resource Cleanup

To delete a complete environment:
1. Go to AWS CloudFormation
2. Delete the corresponding stack (e.g., `endless-image-processor-dev`)
3. This automatically deletes all created resources