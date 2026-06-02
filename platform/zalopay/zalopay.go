package zalopay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// Config holds ZaloPay credentials.
type Config struct {
	AppID       int
	Key1        string
	Key2        string
	Endpoint    string
	CallbackURL string
}

// Client wraps ZaloPay API calls.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New creates a ZaloPay client.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// IsConfigured returns true if credentials are set.
func (c *Client) IsConfigured() bool {
	return c.cfg.AppID != 0 && c.cfg.Key1 != "" && c.cfg.Key2 != ""
}

// GenerateAppTransID generates a unique transaction ID prefixed with Vietnam date yymmdd.
func (c *Client) GenerateAppTransID(bookingID string) string {
	now := time.Now().UTC().Add(7 * time.Hour) // Vietnam GMT+7
	datePrefix := now.Format("060102")
	suffix := strings.ReplaceAll(bookingID, "-", "")
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	randomSuffix := fmt.Sprintf("%03d", rand.Intn(1000))
	return fmt.Sprintf("%s_%s%s", datePrefix, suffix, randomSuffix)
}

// CreateOrderRequest is sent to ZaloPay /v2/create.
type CreateOrderRequest struct {
	AppID                 int    `json:"app_id"`
	AppUser               string `json:"app_user"`
	AppTransID            string `json:"app_trans_id"`
	AppTime               int64  `json:"app_time"`
	ExpireDurationSeconds int64  `json:"expire_duration_seconds,omitempty"`
	Amount                int    `json:"amount"`
	Item                  string `json:"item"`
	Description           string `json:"description"`
	EmbedData             string `json:"embed_data"`
	BankCode              string `json:"bank_code"`
	MAC                   string `json:"mac"`
	CallbackURL           string `json:"callback_url,omitempty"`
}

// CreateOrderResponse from ZaloPay /v2/create.
type CreateOrderResponse struct {
	ReturnCode       int    `json:"return_code"`
	ReturnMessage    string `json:"return_message"`
	SubReturnCode    int    `json:"sub_return_code"`
	SubReturnMessage string `json:"sub_return_message"`
	OrderURL         string `json:"order_url"`
	ZPTransToken     string `json:"zp_trans_token"`
	OrderToken       string `json:"order_token"`
}

// CallbackData parsed from ZaloPay callback.
type CallbackData struct {
	AppID      int    `json:"app_id"`
	AppTransID string `json:"app_trans_id"`
	AppTime    int64  `json:"app_time"`
	AppUser    string `json:"app_user"`
	Amount     int    `json:"amount"`
	EmbedData  string `json:"embed_data"`
	Item       string `json:"item"`
	ZPTransID  int64  `json:"zp_trans_id"`
	ServerTime int64  `json:"server_time"`
	Channel    int    `json:"channel"`
}

// EmbedData stored inside ZaloPay order.
type EmbedData struct {
	BookingID     string `json:"bookingId"`
	MatchPlayerID string `json:"matchPlayerId,omitempty"`
}

// CreateOrderInput contains merchant order data for ZaloPay /v2/create.
type CreateOrderInput struct {
	AppTransID            string
	Description           string
	GuestName             string
	GuestPhone            string
	Amount                int
	EmbedData             EmbedData
	ExpireDurationSeconds int64
}

// QueryOrderResponse from ZaloPay /v2/query.
type QueryOrderResponse struct {
	ReturnCode       int    `json:"return_code"`
	ReturnMessage    string `json:"return_message"`
	SubReturnCode    int    `json:"sub_return_code"`
	SubReturnMessage string `json:"sub_return_message"`
	IsProcessing     bool   `json:"is_processing"`
	Amount           int    `json:"amount"`
	ZPTransID        int64  `json:"zp_trans_id"`
	ServerTime       int64  `json:"server_time"`
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// orderMAC: app_id|app_trans_id|app_user|amount|app_time|embed_data|item
func (c *Client) orderMAC(appTransID, appUser string, amount int, appTime int64, embedData, item string) string {
	input := fmt.Sprintf("%d|%s|%s|%d|%d|%s|%s",
		c.cfg.AppID, appTransID, appUser, amount, appTime, embedData, item)
	return hmacSHA256(c.cfg.Key1, input)
}

// callbackMAC: HMAC-SHA256(key2, data)
func (c *Client) callbackMAC(data string) string {
	return hmacSHA256(c.cfg.Key2, data)
}

// queryMAC: app_id|app_trans_id|key1
func (c *Client) queryMAC(appTransID string) string {
	input := fmt.Sprintf("%d|%s|%s", c.cfg.AppID, appTransID, c.cfg.Key1)
	return hmacSHA256(c.cfg.Key1, input)
}

// CreateOrder creates a ZaloPay order and returns the response.
func (c *Client) CreateOrder(ctx context.Context, input CreateOrderInput) (*CreateOrderResponse, error) {
	appTime := time.Now().UnixMilli()

	// Sanitize app_user (max 50 chars, alphanumeric + underscore)
	appUser := sanitize(input.GuestName+"_"+input.GuestPhone, 50)
	if appUser == "" {
		appUser = "smatch"
	}

	embedDataJSON, _ := json.Marshal(input.EmbedData)
	embedDataStr := string(embedDataJSON)
	item := "[]"

	mac := c.orderMAC(input.AppTransID, appUser, input.Amount, appTime, embedDataStr, item)

	req := CreateOrderRequest{
		AppID:                 c.cfg.AppID,
		AppUser:               appUser,
		AppTransID:            input.AppTransID,
		AppTime:               appTime,
		ExpireDurationSeconds: input.ExpireDurationSeconds,
		Amount:                input.Amount,
		Item:                  item,
		Description:           input.Description,
		EmbedData:             embedDataStr,
		BankCode:              "",
		MAC:                   mac,
	}
	if c.cfg.CallbackURL != "" {
		req.CallbackURL = c.cfg.CallbackURL
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint+"/v2/create", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("zalopay: create order request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zalopay: create order: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result CreateOrderResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("zalopay: parse response: %w", err)
	}
	return &result, nil
}

// VerifyCallback verifies the MAC on a ZaloPay callback and parses the data.
// Returns nil if MAC is invalid.
func (c *Client) VerifyCallback(data, mac string) (*CallbackData, bool) {
	expected := c.callbackMAC(data)
	if mac != expected {
		return nil, false
	}
	var cb CallbackData
	if err := json.Unmarshal([]byte(data), &cb); err != nil {
		return nil, false
	}
	return &cb, true
}

// ExtractEmbedData parses the embed_data field from callback.
func (c *Client) ExtractEmbedData(raw string) (*EmbedData, error) {
	var ed EmbedData
	if err := json.Unmarshal([]byte(raw), &ed); err != nil {
		return nil, err
	}
	return &ed, nil
}

// QueryOrder queries a ZaloPay order status.
func (c *Client) QueryOrder(ctx context.Context, appTransID string) (*QueryOrderResponse, error) {
	payload := map[string]interface{}{
		"app_id":       c.cfg.AppID,
		"app_trans_id": appTransID,
		"mac":          c.queryMAC(appTransID),
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint+"/v2/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result QueryOrderResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func sanitize(s string, maxLen int) string {
	var out strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out.WriteRune(r)
		}
		if out.Len() >= maxLen {
			break
		}
	}
	return out.String()
}
