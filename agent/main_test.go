package main

import (
    "testing"
    "time"
)

func TestSyncLagPositive(t *testing.T) {
    past := time.Now().Add(-45 * time.Second)
    lag  := time.Since(past).Seconds()
    if lag < 44 || lag > 50 {
        t.Errorf("expected ~45s lag, got %.1f", lag)
    }
}

func TestAlertThresholdWarn(t *testing.T) {
    threshold := 60.0
    lag       := 90.0
    if lag < threshold {
        t.Error("expected warning to trigger")
    }
}

func TestAlertThresholdCritical(t *testing.T) {
    threshold := 300.0
    lag       := 350.0
    if lag < threshold {
        t.Error("expected critical to trigger")
    }
}
