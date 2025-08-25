BINARY_NAME=bootstrap
BUCKET_NAME?=my-images-bucket

.PHONY: build clean deploy local-test

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)
	rm -rf .aws-sam/

deploy: build
	sam deploy --guided --parameter-overrides BucketName=$(BUCKET_NAME)

local-test: build
	sam local start-api

package: build
	sam package --s3-bucket $(BUCKET_NAME)-deployments --output-template-file packaged-template.yaml

validate:
	sam validate

init-go:
	go mod tidy

all: clean init-go build validate