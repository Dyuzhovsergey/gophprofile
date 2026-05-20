package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/otel/attribute"
)

// s3Attrs возвращает общие атрибуты для S3/MinIO spans.
func s3Attrs(bucket string, operation string, key string, attrs ...attribute.KeyValue) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs)+3)

	result = append(result,
		attribute.String("s3.bucket", bucket),
		attribute.String("s3.operation", operation),
		attribute.String("s3.key", strings.TrimSpace(key)),
	)

	result = append(result, attrs...)

	return result
}

// Upload сохраняет объект в S3/MinIO.
func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) (err error) {
	key = strings.TrimSpace(key)

	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"s3.upload",
		s3Attrs(
			c.bucket,
			"upload",
			key,
			attribute.String("content_type", strings.TrimSpace(contentType)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if key == "" {
		return ErrEmptyObjectKey
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read s3 object body: %w", err)
	}

	span.SetAttributes(attribute.Int64("file.size", int64(len(data))))

	input := &awss3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
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
func (c *Client) Download(ctx context.Context, key string) (data []byte, contentType string, err error) {
	key = strings.TrimSpace(key)

	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"s3.download",
		s3Attrs(c.bucket, "download", key)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if key == "" {
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

	data, err = io.ReadAll(output.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read s3 object body: %w", err)
	}

	contentType = aws.ToString(output.ContentType)

	span.SetAttributes(
		attribute.String("content_type", contentType),
		attribute.Int64("file.size", int64(len(data))),
	)

	return data, contentType, nil
}

// Delete удаляет объект из S3/MinIO.
func (c *Client) Delete(ctx context.Context, key string) (err error) {
	key = strings.TrimSpace(key)

	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"s3.delete",
		s3Attrs(c.bucket, "delete", key)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if key == "" {
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
func (c *Client) Exists(ctx context.Context, key string) (exists bool, err error) {
	key = strings.TrimSpace(key)

	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"s3.exists",
		s3Attrs(c.bucket, "exists", key)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if key == "" {
		return false, ErrEmptyObjectKey
	}

	if _, err := c.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}); err != nil {
		if isNotFoundError(err) {
			span.SetAttributes(attribute.Bool("s3.exists", false))
			return false, nil
		}

		return false, fmt.Errorf("head s3 object: %w", err)
	}

	span.SetAttributes(attribute.Bool("s3.exists", true))

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

// Ping проверяет доступность S3 bucket.
func (c *Client) Ping(ctx context.Context) error {
	if _, err := c.client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	}); err != nil {
		return fmt.Errorf("head s3 bucket: %w", err)
	}

	return nil
}
