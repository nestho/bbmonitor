package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nestho/bbmonitor/internal/config"
	"github.com/nestho/bbmonitor/internal/storage"
)

type Notifier struct {
	cfg    *config.Config
	client *http.Client
}

func New(cfg *config.Config) *Notifier {
	return &Notifier{cfg: cfg, client: &http.Client{Timeout: 20 * time.Second}}
}

func (n *Notifier) SendChanges(ctx context.Context, changes []storage.Change) error {
	if !n.cfg.Notify.Enabled || len(changes) == 0 {
		return nil
	}
	summary := map[string]int{}
	var details []string
	for _, c := range changes {
		summary[c.Kind]++
		details = append(details, fmt.Sprintf("• [%s] %s — %s", c.Kind, c.Entity, truncate(c.Details, 120)))
	}
	var b strings.Builder
	b.WriteString("🔔 *bbmonitor* — changes detected\n\n")
	for k, v := range summary {
		b.WriteString(fmt.Sprintf("%s: %d\n", k, v))
	}
	b.WriteString("\n")
	if len(details) > 25 {
		b.WriteString(strings.Join(details[:25], "\n"))
		b.WriteString(fmt.Sprintf("\n… and %d more", len(changes)-25))
	} else {
		b.WriteString(strings.Join(details, "\n"))
	}
	msg := b.String()
	var errs []string
	if n.cfg.Notify.Telegram.Enabled && n.cfg.Notify.Telegram.BotToken != "" && n.cfg.Notify.Telegram.ChatID != "" {
		if err := n.sendTelegram(ctx, msg); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}
	if n.cfg.Notify.Webhook.Enabled && n.cfg.Notify.Webhook.URL != "" {
		if err := n.sendWebhook(ctx, changes, msg); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func (n *Notifier) sendTelegram(ctx context.Context, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.Notify.Telegram.BotToken)
	body, _ := json.Marshal(map[string]interface{}{"chat_id": n.cfg.Notify.Telegram.ChatID, "text": text, "parse_mode": "Markdown"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendWebhook(ctx context.Context, changes []storage.Change, text string) error {
	body, _ := json.Marshal(map[string]interface{}{"content": text, "text": text, "changes": changes, "count": len(changes), "source": "bbmonitor"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.Notify.Webhook.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.cfg.Notify.Webhook.Headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
