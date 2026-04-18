package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"
const defaultClientTimeout = 10 * time.Second

type AdapterConfig struct {
	BotToken string
	ChatID   string
}

type Adapter struct {
	botToken string
	chatID   string
	baseURL  string
	client   *http.Client
}

func NewAdapter(cfg AdapterConfig) (*Adapter, error) {
	return newAdapter(cfg, defaultBaseURL, &http.Client{Timeout: defaultClientTimeout})
}

func newAdapter(cfg AdapterConfig, baseURL string, client *http.Client) (*Adapter, error) {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return nil, fmt.Errorf("telegram credentials are required")
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: defaultClientTimeout}
	}
	return &Adapter{
		botToken: cfg.BotToken,
		chatID:   cfg.ChatID,
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   client,
	}, nil
}

func (a *Adapter) Send(ctx context.Context, msg Message) error {
	form := url.Values{}
	form.Set("chat_id", a.chatID)
	form.Set("text", msg.Text)
	if msg.ParseMode != "" {
		form.Set("parse_mode", msg.ParseMode)
	}

	endpoint := a.baseURL + "/bot" + a.botToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create telegram sendMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %s", redactToken(err.Error(), a.botToken))
	}
	defer resp.Body.Close() //nolint:errcheck

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if readErr != nil {
		return fmt.Errorf("read telegram response: %w", readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("telegram sendMessage failed: status %d: %s", resp.StatusCode, redactToken(sanitizeResponse(body), a.botToken))
	}

	var decoded struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			return fmt.Errorf("decode telegram response: %w", err)
		}
		if !decoded.OK {
			return fmt.Errorf("telegram sendMessage rejected: %s", redactToken(sanitizeText(decoded.Description), a.botToken))
		}
	}
	return nil
}

func sanitizeResponse(body []byte) string {
	return sanitizeText(string(body))
}

func sanitizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "empty response"
	}
	return s
}

func redactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}
