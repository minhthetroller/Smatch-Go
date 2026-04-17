package dto

// CreatePaymentRequest from Flutter
type CreatePaymentRequest struct {
	BookingID string `json:"bookingId" validate:"required,uuid"`
}

// QRCodeData in CreatePaymentResponse
type QRCodeData struct {
	Base64    string `json:"base64"`
	RawBase64 string `json:"rawBase64"`
}

// PaymentResponse sent to Flutter
type PaymentResponse struct {
	ID            string  `json:"id"`
	BookingID     *string `json:"bookingId"`
	MatchPlayerID *string `json:"matchPlayerId"`
	PaymentType   string  `json:"paymentType"`
	AppTransID    string  `json:"appTransId"`
	ZPTransID     *string `json:"zpTransId"`
	Amount        int     `json:"amount"`
	Status        string  `json:"status"`
	OrderURL      *string `json:"orderUrl"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

// CreatePaymentResponse sent to Flutter after payment creation
type CreatePaymentResponse struct {
	Payment       PaymentResponse `json:"payment"`
	OrderURL      string          `json:"orderUrl"`
	QRCode        QRCodeData      `json:"qrCode"`
	ZPTransToken  *string         `json:"zpTransToken"`
	ExpireAt      string          `json:"expireAt"`
	WsSubscribeURL string         `json:"wsSubscribeUrl"`
}

// ZaloPayCallbackRequest from ZaloPay webhook
type ZaloPayCallbackRequest struct {
	Data string `json:"data"`
	MAC  string `json:"mac"`
	Type int    `json:"type"`
}

// ZaloPayCallbackResponse sent back to ZaloPay
type ZaloPayCallbackResponse struct {
	ReturnCode    int    `json:"return_code"`
	ReturnMessage string `json:"return_message"`
}

// PaymentStatusResponse sent to Flutter
type PaymentStatusResponse struct {
	Payment   PaymentResponse `json:"payment"`
	IsExpired bool            `json:"isExpired"`
}

// CreateMatchPaymentRequest from Flutter
type CreateMatchPaymentRequest struct {
	MatchPlayerID string `json:"matchPlayerId" validate:"required,uuid"`
}
