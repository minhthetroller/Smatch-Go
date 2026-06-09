package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Config holds AWS S3 settings.
type Config struct {
	Region             string
	AccessKeyID        string
	SecretAccessKey    string
	Endpoint           string // optional: LocalStack override
	BucketProfile      string
	BucketMatches      string
	BucketBusinessDocs string
}

// Client wraps the AWS S3 client.
type Client struct {
	s3                 *s3.Client
	region             string
	BucketProfile      string
	BucketMatches      string
	BucketBusinessDocs string
}

// New creates an S3 client.
func New(ctx context.Context, cfg Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if strings.TrimSpace(cfg.AccessKeyID) != "" && strings.TrimSpace(cfg.SecretAccessKey) != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	return &Client{
		s3:                 s3.NewFromConfig(awsCfg, s3Opts...),
		region:             cfg.Region,
		BucketProfile:      cfg.BucketProfile,
		BucketMatches:      cfg.BucketMatches,
		BucketBusinessDocs: cfg.BucketBusinessDocs,
	}, nil
}

// EnsureBuckets creates buckets that don't yet exist. Idempotent — safe to call on startup.
func (c *Client) EnsureBuckets(ctx context.Context, buckets ...string) error {
	for _, bucket := range buckets {
		if bucket == "" {
			continue
		}
		_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
		if err == nil {
			continue
		}
		if !isBucketNotFound(err) {
			return fmt.Errorf("s3: check bucket %q: %w", bucket, err)
		}

		input := &s3.CreateBucketInput{Bucket: aws.String(bucket)}
		if c.region != "" && c.region != "us-east-1" {
			input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(c.region),
			}
		}
		_, err = c.s3.CreateBucket(ctx, input)
		if err != nil {
			var alreadyOwned *types.BucketAlreadyOwnedByYou
			var alreadyExists *types.BucketAlreadyExists
			if errors.As(err, &alreadyOwned) || errors.As(err, &alreadyExists) {
				continue
			}
			return fmt.Errorf("s3: create bucket %q: %w", bucket, err)
		}
	}
	return nil
}

func isBucketNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchBucket"
}

// PutObject uploads an object to S3.
func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

// PutObjectEncrypted uploads an object to S3 with server-side encryption.
func (c *Client) PutObjectEncrypted(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		Body:                 body,
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	return err
}

// DeleteObject removes an object from S3.
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}
