---
name: live-whatsapp
description: Connect a togo live agent to WhatsApp — outbound replies via the WhatsApp Cloud API and inbound messages via a Meta webhook that feeds live.Ingest.
---

# togo live-whatsapp

Use this skill to let a togo `live` agent converse over WhatsApp. Activate by
blank-importing the driver; `live` auto-registers the `whatsapp` channel.

```go
import _ "github.com/togo-framework/live-whatsapp"
```

## 1. Set env
```bash
export WHATSAPP_TOKEN=...          # Graph API bearer token
export WHATSAPP_PHONE_ID=...       # sender phone-number-id
export WHATSAPP_VERIFY_TOKEN=...   # your webhook handshake secret
# export WHATSAPP_API=https://graph.facebook.com/v21.0   # optional override
```
Missing `WHATSAPP_TOKEN`/`WHATSAPP_PHONE_ID` → outbound is a silent no-op. Never commit these.

## 2. Expose the webhook
- `GET  /api/live/channels/whatsapp/webhook` — Meta verification handshake (echoes `hub.challenge` when `hub.verify_token` == `WHATSAPP_VERIFY_TOKEN`).
- `POST /api/live/channels/whatsapp/webhook` — inbound messages → `live.Ingest("whatsapp", wa_id, body)`.

## 3. Register in Meta
Meta app → WhatsApp → Configuration → Webhooks: Callback URL = your public origin + the path above; Verify token = `WHATSAPP_VERIFY_TOKEN`; subscribe to `messages`.

## Send / receive
- **Receive:** inbound text routes by `wa_id` — new `wa_id` opens a conversation (`ExternalRef = wa_id`, agent = `LIVE_CHANNEL_AGENT` or first registered); repeats reuse it.
- **Send:** the agent reply is delivered via `POST {api}/{phoneID}/messages` (`type: text`) to `conv.ExternalRef`.

## Notes
- Only plain text is sent; replies must be within WhatsApp's 24h session window (older ones need approved templates, not supported here).
- Webhook always returns 200 (Meta retries non-2xx); non-text events are ignored.
