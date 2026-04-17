package firebase

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client wraps Firebase Auth and Messaging.
type Client struct {
	Auth      *auth.Client
	Messaging *messaging.Client
}

// New initialises a Firebase client from a service account credentials file.
//
// credentialsFile must be an absolute or relative path to the Admin SDK JSON
// downloaded from the Firebase console (e.g. "firebase-adminsdk.json").
// The file is read once at startup; its contents are never written to disk again.
func New(ctx context.Context, credentialsFile string) (*Client, error) {
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("firebase: read credentials file %q: %w", credentialsFile, err)
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(raw))
	if err != nil {
		return nil, fmt.Errorf("firebase: new app: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: auth client: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: messaging client: %w", err)
	}

	return &Client{Auth: authClient, Messaging: msgClient}, nil
}

// VerifyIDToken verifies a Firebase ID token and returns the decoded token.
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if c == nil || c.Auth == nil {
		return nil, fmt.Errorf("firebase: client not initialised (missing credentials)")
	}
	return c.Auth.VerifyIDToken(ctx, idToken)
}

// SendMulticast sends an FCM notification to multiple device tokens.
func (c *Client) SendMulticast(ctx context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	if c == nil || c.Messaging == nil {
		return nil, fmt.Errorf("firebase: client not initialised (missing credentials)")
	}
	return c.Messaging.SendEachForMulticast(ctx, msg)
}
