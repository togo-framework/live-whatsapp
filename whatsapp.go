// Package whatsapp is a WhatsApp channel driver for the togo live plugin. It
// delivers agent replies out through the WhatsApp Cloud (Graph) API and ingests
// inbound messages via a Meta webhook (see webhook.go).
//
// Configure with env: WHATSAPP_TOKEN (Graph API bearer), WHATSAPP_PHONE_ID
// (sender phone number id), WHATSAPP_API (default
// https://graph.facebook.com/v21.0) and WHATSAPP_VERIFY_TOKEN (webhook
// verification handshake secret). Activate by blank-importing this package.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/togo-framework/live"
	"github.com/togo-framework/togo"
)

const defaultAPI = "https://graph.facebook.com/v21.0"

func init() {
	live.RegisterChannel("whatsapp", func(k *togo.Kernel) live.Channel {
		return &channel{
			token:   os.Getenv("WHATSAPP_TOKEN"),
			phoneID: os.Getenv("WHATSAPP_PHONE_ID"),
			api:     env("WHATSAPP_API", defaultAPI),
			client:  &http.Client{Timeout: 15 * time.Second},
		}
	})
}

type channel struct {
	token   string
	phoneID string
	api     string
	client  *http.Client
}

// Name is the conversation.channel value this driver handles.
func (c *channel) Name() string { return "whatsapp" }

// Deliver pushes an agent reply out to the WhatsApp thread identified by
// conv.ExternalRef (the recipient wa_id / msisdn). It is a no-op when the driver
// is unconfigured (mirrors the slack channel when its webhook is empty).
func (c *channel) Deliver(ctx context.Context, conv live.Conversation, msg live.Message) error {
	if c.token == "" || c.phoneID == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                conv.ExternalRef,
		"type":              "text",
		"text":              map[string]any{"body": msg.Body},
	})
	url := c.api + "/" + c.phoneID + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp: %s", resp.Status)
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
