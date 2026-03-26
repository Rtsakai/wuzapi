package main

import "time"

func configuredSentRevokeMaxAge() time.Duration {
	if revokeSentMaxAgeMinutes == nil || *revokeSentMaxAgeMinutes <= 0 {
		return 0
	}
	return time.Duration(*revokeSentMaxAgeMinutes) * time.Minute
}

func isTrackedMessageWithinRevokeWindow(sentAt, now time.Time, maxAge time.Duration) bool {
	if maxAge <= 0 {
		return true
	}
	if sentAt.IsZero() {
		return true
	}
	return !now.After(sentAt.Add(maxAge))
}
