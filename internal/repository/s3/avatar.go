package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// Upload сохраняет объект в S3/MinIO.
func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	if strings.TrimSpace(key) == "" {
		return ErrEmptyObjectKey
	}

	input := &awss3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	if strings.TrimSpace(contentType) != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := c.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}

	return nil
}

// Download скачивает объект из S3/MinIO и возвращает его содержимое и Content-Type.
func (c *Client) Download(ctx context.Context, key string) ([]byte, string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, "", ErrEmptyObjectKey
	}

	output, err := c.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get s3 object: %w", err)
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read s3 object body: %w", err)
	}

	return data, aws.ToString(output.ContentType), nil
}

// Delete удаляет объект из S3/MinIO.
func (c *Client) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrEmptyObjectKey
	}

	if _, err := c.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}

	return nil
}

// Exists проверяет существование объекта в S3/MinIO.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, ErrEmptyObjectKey
	}

	if _, err := c.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}); err != nil {
		if isNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("head s3 object: %w", err)
	}

	return true, nil
}

// isNotFoundError проверяет, что S3 вернул ошибку отсутствующего объекта.
func isNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	switch apiErr.ErrorCode() {
	case "NotFound", "NoSuchKey", "NoSuchBucket":
		return true
	default:
		return false
	}
}
