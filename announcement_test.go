package main

import (
	"testing"
	"time"
)

func TestAnnouncementEventDecision(t *testing.T) {
	deadline := time.Date(2026, 10, 2, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		input   Announcement
		want    string
		wantErr bool
	}{
		{"course delivery", Announcement{Kind: KindCourseDelivery, Title: "Module 4 open", Body: "The lab is ready.", CourseID: "course-42"}, "course.delivery", false},
		{"learner deadline", Announcement{Kind: KindLearnerDeadline, Title: "Lab due", Body: "Submit the lab.", CourseID: "course-42", DueAt: deadline}, "learner.deadline", false},
		{"deadline needs time", Announcement{Kind: KindLearnerDeadline, Title: "Lab due", Body: "Submit the lab.", CourseID: "course-42"}, "", true},
		{"educator report", Announcement{Kind: KindEducatorReport, Title: "Weekly report", Body: "Completion report posted.", CourseID: "course-42"}, "educator.report", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.event()
			if (err != nil) != tt.wantErr {
				t.Fatalf("event() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("event() = %q, want %q", got, tt.want)
			}
		})
	}
}
