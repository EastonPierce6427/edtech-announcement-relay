package main

import (
	"errors"
	"strings"
	"time"
)

const (
	KindCourseDelivery  = "course_delivery"
	KindLearnerDeadline = "learner_deadline"
	KindEducatorReport  = "educator_report"
)

type Announcement struct {
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CourseID  string    `json:"course_id"`
	DueAt     time.Time `json:"due_at,omitempty"`
	AccountID string    `json:"account_id,omitempty"`
}

type Delivery struct {
	Channel        string `json:"channel"`
	Event          string `json:"event"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (a Announcement) event() (string, error) {
	if strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Body) == "" || strings.TrimSpace(a.CourseID) == "" {
		return "", errors.New("title, body, and course_id are required")
	}

	switch a.Kind {
	case KindCourseDelivery:
		return "course.delivery", nil
	case KindLearnerDeadline:
		if a.DueAt.IsZero() {
			return "", errors.New("due_at is required for learner deadlines")
		}
		return "learner.deadline", nil
	case KindEducatorReport:
		return "educator.report", nil
	default:
		return "", errors.New("kind must be course_delivery, learner_deadline, or educator_report")
	}
}
