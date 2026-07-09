package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/togo-framework/live"
	"github.com/togo-framework/togo"
)

const webhookPath = "/api/live/channels/whatsapp/webhook"

func init() {
	// PriorityLate+30: mount after every plugin + any auth middleware (chi
	// requires middleware to be registered before routes).
	togo.RegisterProviderFunc("live-whatsapp-webhook", togo.PriorityLate+30, func(k *togo.Kernel) error {
		if k.Router == nil {
			return nil
		}
		k.Router.Get(webhookPath, verifyHandler)
		k.Router.Post(webhookPath, inboundHandler(k))
		return nil
	})
}

// verifyHandler answers Meta's subscription verification handshake: echo back
// hub.challenge when hub.mode is "subscribe" and hub.verify_token matches
// WHATSAPP_VERIFY_TOKEN, otherwise 403.
func verifyHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hub.mode") == "subscribe" && q.Get("hub.verify_token") == os.Getenv("WHATSAPP_VERIFY_TOKEN") {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(q.Get("hub.challenge")))
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

// inbound webhook payload — only the fields we read.
type webhookPayload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					From string `json:"from"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// inboundHandler parses a Meta webhook delivery and ingests each inbound text
// message as a user prompt. It always replies 200 quickly — Meta retries on any
// non-2xx — and ignores status callbacks / non-text events gracefully.
func inboundHandler(k *togo.Kernel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer w.WriteHeader(http.StatusOK)

		svc, ok := live.FromKernel(k)
		if !ok {
			return
		}
		var p webhookPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return
		}
		ctx := context.Background()
		for _, e := range p.Entry {
			for _, ch := range e.Changes {
				for _, m := range ch.Value.Messages {
					if m.Type != "text" || m.From == "" || m.Text.Body == "" {
						continue
					}
					_, _ = svc.Ingest(ctx, "whatsapp", m.From, m.Text.Body)
				}
			}
		}
	}
}
