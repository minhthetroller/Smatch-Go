package blob

import (
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

// Config holds Azure Blob Storage settings.
type Config struct {
	AccountName          string
	AccountKey           string
	Endpoint             string // optional: Azurite override
	ContainerProfile     string
	ContainerMatches     string
	ContainerBusinessDocs string
}

// Client wraps the Azure Blob Storage client.
type Client struct {
	client              *azblob.Client
	ContainerProfile     string
	ContainerMatches     string
	ContainerBusinessDocs string
	AccountName          string
}

// New creates an Azure Blob Storage client.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AccountName == "" {
		return nil, fmt.Errorf("blob: account name is required")
	}

	serviceURL := cfg.Endpoint
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net", cfg.AccountName)
	}

	cred, err := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("blob: create credential: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("blob: create client: %w", err)
	}

	return &Client{
		client:               client,
		ContainerProfile:     cfg.ContainerProfile,
		ContainerMatches:     cfg.ContainerMatches,
		ContainerBusinessDocs: cfg.ContainerBusinessDocs,
		AccountName:          cfg.AccountName,
	}, nil
}

// PutObject uploads an object to Blob Storage.
func (c *Client) PutObject(ctx context.Context, container, key string, body io.Reader, contentType string) error {
	opts := &azblob.UploadStreamOptions{}
	if contentType != "" {
		opts.HTTPHeaders = &blob.HTTPHeaders{
			BlobContentType: &contentType,
		}
	}
	_, err := c.client.UploadStream(ctx, container, key, body, opts)
	return err
}

// PutObjectEncrypted uploads an object to Blob Storage.
// Azure Blob Storage encrypts all data at rest with service-managed keys by default,
// so this behaves identically to PutObject.
func (c *Client) PutObjectEncrypted(ctx context.Context, container, key string, body io.Reader, contentType string) error {
	return c.PutObject(ctx, container, key, body, contentType)
}

// DeleteObject removes an object from Blob Storage.
func (c *Client) DeleteObject(ctx context.Context, container, key string) error {
	_, err := c.client.DeleteBlob(ctx, container, key, nil)
	return err
}
