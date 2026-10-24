package app

import (
	"testing"
	"time"
)

func TestSlidingWindow(t *testing.T) {
	limit := newSliding(2, time.Minute)
	if !limit.allow("a") || !limit.allow("a") || limit.allow("a") {
		t.Fatal("limit did not trip on the third hit")
	}
	if !limit.allow("b") {
		t.Fatal("different key was blocked")
	}
}
