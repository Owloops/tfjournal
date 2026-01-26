package run

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	ts := time.Date(2025, 1, 26, 14, 30, 52, 0, time.UTC)
	id := GenerateID(ts)

	if !strings.HasPrefix(id, "run_20250126T143052_") {
		t.Errorf("ID = %s, want prefix run_20250126T143052_", id)
	}
	if len(id) != 28 {
		t.Errorf("ID length = %d, want 28", len(id))
	}
}

func TestGenerateID_Unique(t *testing.T) {
	ts := time.Now()
	id1 := GenerateID(ts)
	id2 := GenerateID(ts)

	if id1 == id2 {
		t.Errorf("generated IDs should be unique, got %s twice", id1)
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid", "run_20250126T143052_a1b2c3d4", false},
		{"valid_another", "run_20251231T235959_deadbeef", false},
		{"old_format", "run_abc12345", true},
		{"too_short", "run_", true},
		{"no_prefix", "20250126T143052_a1b2c3d4", true},
		{"bad_timestamp", "run_2025012T143052_a1b2c3d4", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestParseDateFromID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantDate string
		wantErr  bool
	}{
		{"valid", "run_20250126T143052_a1b2c3d4", "2025-01-26", false},
		{"valid_another", "run_20251231T235959_deadbeef", "2025-12-31", false},
		{"old_format", "run_abc12345", "", true},
		{"too_short", "run_", "", true},
		{"no_prefix", "20250126T143052_a1b2c3d4", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateFromID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateFromID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Format("2006-01-02") != tt.wantDate {
				t.Errorf("ParseDateFromID(%q) = %s, want %s", tt.id, got.Format("2006-01-02"), tt.wantDate)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if err != nil {
				t.Errorf("ParseDuration(%q) error = %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
