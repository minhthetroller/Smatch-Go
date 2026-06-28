# Payment WebSocket Frontend Guide

This payment flow is websocket-first. The frontend must not poll `GET /api/payments/{id}/status`; that endpoint has been removed. After creating a payment, subscribe to the websocket URL returned by the backend and wait for `payment_status` events.

## Create Payment

Booking payment:

```http
POST /api/payments/create
```

Match join payment:

```http
POST /api/matches/{matchId}/payment
```

Both responses include `wsSubscribeUrl`:

```json
{
  "success": true,
  "data": {
    "payment": {
      "id": "payment-id",
      "status": "pending"
    },
    "orderUrl": "https://...",
    "qrCode": {
      "base64": "data:image/png;base64,...",
      "rawBase64": "..."
    },
    "zpTransToken": "token",
    "expireAt": "2026-05-21T10:00:00.000Z",
    "wsSubscribeUrl": "wss://api.example.com/ws/payments?paymentId=payment-id&ticket=single-use-ticket"
  }
}
```

`expireAt` is always exactly 5 minutes after the backend payment record was created. Treat the QR/order URL as invalid after that timestamp.

Use `wsSubscribeUrl` exactly as returned. It already contains:

- `paymentId`
- a short-lived, single-use `ticket`
- the correct `ws` or `wss` scheme
- the correct backend host

## Ticket Rules

The websocket ticket is short-lived and single-use.

- Open the websocket immediately after receiving `wsSubscribeUrl`.
- Do not store or reuse the URL after the current payment screen/session.
- After the backend accepts the ticket, it deletes the ticket from Redis.
- If the socket connection is not opened before the ticket expires, call the create-payment endpoint again. For booking payments, the backend can return the existing pending payment with a fresh `wsSubscribeUrl`.
- Do not share one payment websocket between multiple app instances. The backend allows only one active connection per payment.

## Subscribe Flow

Preferred flow: connect directly to `wsSubscribeUrl`.

```ts
const socket = new WebSocket(createPaymentResponse.data.wsSubscribeUrl);

socket.onmessage = (event) => {
  const message = JSON.parse(event.data);
  handlePaymentSocketMessage(message);
};
```

The backend auto-subscribes when `paymentId` and `ticket` are present in the query string. The frontend does not need to send a manual subscribe message in the normal flow.

Manual subscribe is still supported only if both fields are provided:

```json
{
  "action": "subscribe",
  "paymentId": "payment-id",
  "ticket": "single-use-ticket"
}
```

## Incoming Messages

Control messages:

```json
{
  "type": "connected",
  "message": "Connected to payment notification service"
}
```

```json
{
  "type": "subscribed",
  "paymentId": "payment-id"
}
```

Ignore these for payment outcome logic.

Payment status message:

```json
{
  "type": "payment_status",
  "paymentId": "payment-id",
  "status": "pending",
  "bookingId": "booking-id",
  "matchPlayerId": null,
  "zpTransId": null,
  "message": "Payment is pending"
}
```

`payment_status` is the source of truth for the payment UI.

Possible `status` values:

- `pending`: payment is still waiting.
- `success`: payment completed.
- `failed`: payment failed or was cancelled.
- `expired`: payment timed out.

The backend sends a DB-backed snapshot immediately after successful subscription. This protects the frontend from missing a callback that arrived before the websocket connected.

## Terminal States

Close the websocket when one of these statuses arrives:

- `success`
- `failed`
- `expired`

Example:

```ts
function handlePaymentSocketMessage(message: any) {
  if (message.type !== "payment_status") {
    return;
  }

  switch (message.status) {
    case "pending":
      renderPendingPayment(message);
      return;
    case "success":
      renderPaymentSuccess(message);
      socket.close();
      return;
    case "failed":
      renderPaymentFailed(message);
      socket.close();
      return;
    case "expired":
      renderPaymentExpired(message);
      socket.close();
      return;
  }
}
```

## Error Messages

Invalid, expired, missing, mismatched, or already-used ticket:

```json
{
  "type": "error",
  "code": "INVALID_PAYMENT_WS_TICKET",
  "paymentId": "payment-id",
  "message": "Invalid or expired payment websocket ticket"
}
```

Frontend handling:

- Close this socket.
- Recreate or refresh the payment by calling the create-payment endpoint again.
- Use the new `wsSubscribeUrl`.

Second active connection for the same payment:

```json
{
  "type": "error",
  "code": "PAYMENT_ALREADY_SUBSCRIBED",
  "paymentId": "payment-id",
  "message": "Payment already has an active websocket subscriber"
}
```

Frontend handling:

- Do not open duplicate sockets for one payment.
- Keep websocket ownership in the payment screen/state manager.
- If this happens after navigation or hot reload, close the new socket and keep the existing active one.

Same websocket trying to subscribe to another payment:

```json
{
  "type": "error",
  "code": "PAYMENT_CONNECTION_ALREADY_SUBSCRIBED",
  "paymentId": "payment-id",
  "message": "WebSocket connection is already subscribed to another payment"
}
```

Frontend handling:

- Use one websocket connection per payment.
- Close the old socket before starting a new payment flow.

## Reconnect Policy

Use a conservative reconnect policy:

- If the socket closes before any terminal `payment_status`, call the create-payment endpoint again to get a fresh `wsSubscribeUrl`.
- Do not reconnect using the old `wsSubscribeUrl`; its ticket may already be consumed.
- Do not poll `/api/payments/{id}/status`.
- Continue using the payment `expireAt` countdown for UI display only. The backend will notify `expired` through websocket.

## Frontend Checklist

- Remove polling calls to `GET /api/payments/{id}/status`.
- Open `data.wsSubscribeUrl` immediately after creating a payment.
- Treat `payment_status` as the only source of truth for payment state.
- Ignore `connected`, `subscribed`, and `pong` for payment outcome logic.
- Close the socket on `success`, `failed`, or `expired`.
- Keep exactly one active websocket connection per payment.
- On ticket errors or early socket close, request a fresh `wsSubscribeUrl` from the create-payment endpoint.
