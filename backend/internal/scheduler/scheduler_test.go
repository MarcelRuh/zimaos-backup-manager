package scheduler

import "testing"

func TestPresetCron(t *testing.T) {
	daily, err := PresetCron("daily")
	if err != nil || daily != "0 3 * * *" {
		t.Fatalf("daily: %q %v", daily, err)
	}
	weekly, err := PresetCron("weekly")
	if err != nil || weekly != "0 3 * * 1" {
		t.Fatalf("weekly: %q %v", weekly, err)
	}
	monthly, err := PresetCron("monthly")
	if err != nil || monthly != "0 3 1 * *" {
		t.Fatalf("monthly: %q %v", monthly, err)
	}
	if _, err := PresetCron("hourly"); err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

func TestValidateCron(t *testing.T) {
	if err := ValidateCron("*/5 * * * *"); err != nil {
		t.Fatalf("valid cron rejected: %v", err)
	}
	if err := ValidateCron("not a cron"); err == nil {
		t.Fatal("expected invalid cron error")
	}
}
