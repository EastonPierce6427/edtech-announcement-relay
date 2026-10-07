package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublisherRetriesRateLimitWithSameKey(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "delivery-course-42-v4" {
			t.Fatalf("idempotency key = %q", got)
		}
		status, body := http.StatusOK, `{"ok":true,"data":{"published":true},"error":null,"metadata":{}}`
		header := make(http.Header)
		if requests == 1 {
			status = http.StatusTooManyRequests
			body = `{"ok":false,"data":null,"error":{"code":"rate_limited","message":"retry later"},"metadata":{}}`
			header.Set("Retry-After", "2")
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}

	var slept time.Duration
	publisher := Publisher{
		APIKey: "test-key", Channel: "edtech-members", HTTPClient: client, MaxRetries: 2,
		Sleep: func(_ context.Context, d time.Duration) error { slept = d; return nil },
	}
	delivery, err := publisher.Publish(context.Background(), Announcement{
		Kind: KindCourseDelivery, Title: "Module 4 open", Body: "The lab is ready.", CourseID: "course-42",
	}, "delivery-course-42-v4")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || slept != 2*time.Second {
		t.Fatalf("requests = %d, slept = %s", requests, slept)
	}
	if delivery.Event != "course.delivery" || delivery.Channel != "edtech-members" {
		t.Fatalf("delivery = %+v", delivery)
	}
}
