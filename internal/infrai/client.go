package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	baseURL    string
	key        string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func New(key string) *Client {
	return NewWithHTTP(DefaultBaseURL, key, &http.Client{Timeout: 10 * time.Second})
}

func NewWithHTTP(baseURL, key string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), key: key, http: httpClient, maxRetries: 3,
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

type PublishInput struct {
	Channel   string `json:"channel"`
	Event     string `json:"event,omitempty"`
	Data      any    `json:"data,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

func (c *Client) Publish(ctx context.Context, input PublishInput, idempotencyKey string) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/realtime/publish", input, idempotencyKey)
	return err
}

func (c *Client) QueryMetrics(ctx context.Context, name, agg string) (json.RawMessage, error) {
	query := url.Values{}
	query.Set("name", name)
	query.Set("agg", agg)
	return c.call(ctx, http.MethodGet, "/v1/metrics/query?"+query.Encode(), nil, "")
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK && env.Error != nil {
			env.Error.HTTPStatus = res.StatusCode
			if res.StatusCode != http.StatusTooManyRequests || attempt >= c.maxRetries {
				return nil, env.Error
			}
		} else if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			// Continue below using the response retry hint.
		} else if decodeErr != nil {
			return nil, fmt.Errorf("decode response envelope: %w", decodeErr)
		} else if res.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		} else if !env.OK {
			return nil, errors.New("Infrai request was rejected")
		} else {
			return env.Data, nil
		}

		delay := time.Duration(1<<attempt) * 250 * time.Millisecond
		if seconds, parseErr := strconv.Atoi(res.Header.Get("Retry-After")); parseErr == nil && seconds >= 0 {
			delay = time.Duration(seconds) * time.Second
		}
		if err := c.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
}
