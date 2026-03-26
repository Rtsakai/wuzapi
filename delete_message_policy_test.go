package main

import (
	"testing"
	"time"
)

func TestIsTrackedMessageWithinRevokeWindow(t *testing.T) {
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if !isTrackedMessageWithinRevokeWindow(now.Add(-30*time.Minute), now, time.Hour) {
		t.Fatalf("expected message inside configured window to be accepted")
	}

	if isTrackedMessageWithinRevokeWindow(now.Add(-2*time.Hour), now, time.Hour) {
		t.Fatalf("expected message older than configured window to be rejected")
	}

	if !isTrackedMessageWithinRevokeWindow(time.Time{}, now, time.Hour) {
		t.Fatalf("expected zero timestamp to skip local age validation")
	}

	if !isTrackedMessageWithinRevokeWindow(now.Add(-24*time.Hour), now, 0) {
		t.Fatalf("expected disabled local window to defer to WhatsApp server policy")
	}
}
