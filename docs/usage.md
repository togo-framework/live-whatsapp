# live-whatsapp — usage

`live-whatsapp` is a WhatsApp channel driver for the togo [`live`](https://to-go.dev/plugins/live)
plugin. It lets a live agent converse over WhatsApp: outbound via the WhatsApp
Cloud (Graph) API, inbound via a Meta webhook that feeds messages into
`live.Ingest`.

Activate it by blank-importing the package — `live` picks up the registered
`whatsapp` channel automatically:

```go
import _ "github.com/togo-framework/live-whatsapp"
```

## 1. Environment

| Env | Required | Default | Description |
|---|---|---|---|
| `WHATSAPP_TOKEN` | yes | — | Graph API bearer token for the WhatsApp Business account. |
| `WHATSAPP_PHONE_ID` | yes | — | Sender phone number id (the WhatsApp Cloud API phone-number-id). |
| `WHATSAPP_API` | no | `https://graph.facebook.com/v21.0` | Graph API base URL — override to pin a different Graph version. |
| `WHATSAPP_VERIFY_TOKEN` | yes (inbound) | — | Shared secret echoed back during Meta's webhook verification handshake. |

If `WHATSAPP_TOKEN` or `WHATSAPP_PHONE_ID` is empty, `Deliver` is a no-op (the
driver degrades gracefully rather than erroring), so outbound is silently
disabled until both are set.

## 2. Outbound (egress)

When the agent produces a reply, the `live` loop calls `Channel.Deliver`. This
driver POSTs a text message to the WhatsApp Cloud API:

```
POST {WHATSAPP_API}/{WHATSAPP_PHONE_ID}/messages
Authorization: Bearer {WHATSAPP_TOKEN}
Content-Type: application/json

{
  "messaging_product": "whatsapp",
  "to": "<conv.ExternalRef>",
  "type": "text",
  "text": { "body": "<agent reply>" }
}
```

- The recipient (`to`) is the conversation's `ExternalRef` — the customer's
  `wa_id` / msisdn, set when the conversation was first opened by an inbound
  message.
- A non-2xx response returns an error (`whatsapp: <status>`); the HTTP client
  uses a 15s timeout.

## 3. Inbound (ingress)

The plugin mounts two routes on the app router (registered late — after every
plugin and any auth middleware):

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/live/channels/whatsapp/webhook` | Meta subscription verification handshake |
| `POST` | `/api/live/channels/whatsapp/webhook` | Inbound message delivery |

### Register the webhook in Meta

In your Meta app dashboard (WhatsApp → Configuration → Webhooks), set the
**Callback URL** to your app's public URL plus the path above, e.g.
`https://your-app.example.com/api/live/channels/whatsapp/webhook`, and set the
**Verify token** to the same value as `WHATSAPP_VERIFY_TOKEN`. Subscribe to the
`messages` field.

### GET verification handshake

When you save the webhook, Meta issues a `GET` with `hub.mode=subscribe`,
`hub.verify_token=...` and `hub.challenge=...`. The `GET` handler echoes back
`hub.challenge` verbatim (200, `text/plain`) only when `hub.mode` is
`subscribe` **and** `hub.verify_token` matches `WHATSAPP_VERIFY_TOKEN`;
otherwise it returns `403`. Set `WHATSAPP_VERIFY_TOKEN` before saving the
webhook or verification will fail.

### POST inbound delivery

Meta `POST`s each inbound event to the same path. The handler:

1. Looks up the `live` service (`live.FromKernel`); if `live` isn't installed it
   replies 200 and does nothing.
2. Decodes the payload and iterates `entry[].changes[].value.messages[]`.
3. Skips anything that isn't a plain text message (non-`text` types, status
   callbacks, empty `from`/`body` are ignored).
4. For each text message, calls
   `svc.Ingest(ctx, "whatsapp", m.From, m.Text.Body)`.

`live.Ingest` routes by `(channel, externalRef)` = `("whatsapp", wa_id)`: an
existing conversation for that `wa_id` is reused, otherwise a new conversation
is opened against the channel's default agent (`LIVE_CHANNEL_AGENT`, else the
first registered agent) with `ExternalRef = wa_id`. The user message becomes a
prompt; the agent's reply is later delivered back out through `Deliver` above.

The handler always replies `200` quickly — Meta retries on any non-2xx, so
errors are swallowed and non-text events are ignored gracefully.

## Notes

- **24-hour session window** — WhatsApp only allows free-form messages within 24
  hours of the user's last inbound message. Outside that window you must send a
  pre-approved template; this driver sends plain `text` messages only.
- **`LIVE_CHANNEL_AGENT`** picks which agent new WhatsApp conversations route to;
  without it the first registered agent is used.
- Never commit `WHATSAPP_TOKEN` or `WHATSAPP_VERIFY_TOKEN` — supply them via env.
