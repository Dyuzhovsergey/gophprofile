package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	// ErrEmptyEndpoint означает, что не указан endpoint S3/MinIO.
	ErrEmptyEndpoint = errors.New("s3 endpoint is empty")

	// ErrEmptyBucket означает, что не указан bucket S3/MinIO.
	ErrEmptyBucket = errors.New("s3 bucket is empty")

	// ErrEmptyObjectKey означает, что не указан ключ объекта в S3/MinIO.
	ErrEmptyObjectKey = errors.New("s3 object key is empty")
)

// Client работает с S3-совместимым хранилищем файлов.
type Client struct {
	client *awss3.Client
	bucket string
}

// NewClient создаёт клиент для работы с S3/MinIO.
func NewClient(ctx context.Context, cfg config.S3Config) (*Client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, ErrEmptyEndpoint
	}

	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, ErrEmptyBucket
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := awss3.NewFromConfig(awsCfg, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = cfg.UsePathStyle
	})

	return &Client{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}
