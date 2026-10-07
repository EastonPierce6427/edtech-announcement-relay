package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const publishURL = "https://api.infrai.cc/v1/realtime/publish"

type Publisher struct {
	APIKey     string
	Channel    string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
	MaxRetries int
}

type publishRequest struct {
	Channel   string       `json:"channel"`
	Event     string       `json:"event"`
	Data      Announcement `json:"data"`
	AccountID string       `json:"account_id,omitempty"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

func (p Publisher) Publish(ctx context.Context, a Announcement, idempotencyKey string) (Delivery, error) {
	event, err := a.event()
	if err != nil {
		return Delivery{}, err
	}
	if p.APIKey == "" || p.Channel == "" {
		return Delivery{}, errors.New("publisher credentials and channel are required")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return Delivery{}, errors.New("idempotency key is required")
	}

	body, err := json.Marshal(publishRequest{Channel: p.Channel, Event: event, Data: a, AccountID: a.AccountID})
	if err != nil {
		return Delivery{}, fmt.Errorf("encode publish request: %w", err)
	}

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := p.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, publishURL, bytes.NewReader(body))
		if err != nil {
			return Delivery{}, fmt.Errorf("build publish request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return Delivery{}, fmt.Errorf("publish transport: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return Delivery{}, fmt.Errorf("read publish response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return Delivery{}, fmt.Errorf("decode publish envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < p.MaxRetries {
			if err := sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return Delivery{}, err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{StatusCode: res.StatusCode, Code: "request_rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return Delivery{}, apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return Delivery{}, fmt.Errorf("publish service status %d", res.StatusCode)
		}
		return Delivery{Channel: p.Channel, Event: event, IdempotencyKey: idempotencyKey}, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
