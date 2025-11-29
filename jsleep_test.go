package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"5", 5 * time.Second, false},
		{"100", 100 * time.Second, false},
		{"100ms", 100 * time.Millisecond, false},
		{"1m", time.Minute, false},
		{"1h", time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"0.5d", 12 * time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"", 0, true},
		{"abc", 0, true},
		{"d", 0, true},
		{"1e308d", 0, true},
		{"-1e308d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePercent(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"25%", 0.25, false},
		{"0%", 0.0, false},
		{"100%", 1.0, false},
		{"50%", 0.5, false},
		{"12.5%", 0.125, false},
		{"50", 0, true},
		{"-10%", 0, true},
		{"abc%", 0, true},
		{"%", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePercent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePercent(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parsePercent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantLow time.Duration
		wantHi  time.Duration
		wantErr bool
	}{
		{
			name:    "default 50% jitter",
			args:    []string{"10s"},
			wantLow: 5 * time.Second,
			wantHi:  15 * time.Second,
		},
		{
			name:    "explicit jitter flag",
			args:    []string{"-j", "20%", "10s"},
			wantLow: 8 * time.Second,
			wantHi:  12 * time.Second,
		},
		{
			name:    "positional jitter",
			args:    []string{"10s", "20%"},
			wantLow: 8 * time.Second,
			wantHi:  12 * time.Second,
		},
		{
			name:    "range jitter",
			args:    []string{"-r", "2s", "10s"},
			wantLow: 8 * time.Second,
			wantHi:  12 * time.Second,
		},
		{
			name:    "bounds only",
			args:    []string{"--min", "5s", "--max", "15s"},
			wantLow: 5 * time.Second,
			wantHi:  15 * time.Second,
		},
		{
			name:    "zero jitter",
			args:    []string{"-j", "0%", "10s"},
			wantLow: 10 * time.Second,
			wantHi:  10 * time.Second,
		},
		{
			name:    "missing duration",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "jitter and range conflict",
			args:    []string{"-j", "20%", "-r", "2s", "10s"},
			wantErr: true,
		},
		{
			name:    "max less than min",
			args:    []string{"--min", "10s", "--max", "5s"},
			wantErr: true,
		},
		{
			name:    "positional and flag jitter conflict",
			args:    []string{"-j", "20%", "10s", "30%"},
			wantErr: true,
		},
		{
			name:    "range without base",
			args:    []string{"-r", "2s"},
			wantErr: true,
		},
		{
			name:    "min/max conflict with base",
			args:    []string{"--min", "10s", "--max", "5s", "10s"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			low, high, _, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if low != tt.wantLow {
				t.Errorf("parseArgs(%v) low = %v, want %v", tt.args, low, tt.wantLow)
			}
			if high != tt.wantHi {
				t.Errorf("parseArgs(%v) high = %v, want %v", tt.args, high, tt.wantHi)
			}
		})
	}
}

func TestParseArgsInvariants(t *testing.T) {
	validArgSets := [][]string{
		{"10s"},
		{"10s", "25%"},
		{"-j", "30%", "5s"},
		{"-r", "1s", "10s"},
		{"--min", "1s", "--max", "10s"},
		{"--min", "5s", "10s"},
		{"--max", "15s", "10s"},
		{"--min", "8s", "--max", "12s", "10s"},
	}

	for _, args := range validArgSets {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			low, high, _, err := parseArgs(args)
			if err != nil {
				t.Errorf("parseArgs(%v) unexpected error: %v", args, err)
				return
			}
			if low > high {
				t.Errorf("parseArgs(%v) low=%v > high=%v", args, low, high)
			}
			if low < 0 {
				t.Errorf("parseArgs(%v) low=%v < 0", args, low)
			}
			if high < 0 {
				t.Errorf("parseArgs(%v) high=%v < 0", args, high)
			}
		})
	}
}

func TestChooseSleepDuration(t *testing.T) {
	t.Run("equal bounds", func(t *testing.T) {
		got, err := chooseSleepDuration(5*time.Second, 5*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 5*time.Second {
			t.Errorf("got %v, want 5s", got)
		}
	})

	t.Run("in bounds", func(t *testing.T) {
		low := 5 * time.Second
		high := 15 * time.Second
		for i := 0; i < 100; i++ {
			got, err := chooseSleepDuration(low, high)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got < low || got > high {
				t.Errorf("iteration %d: got %v, want in [%v, %v]", i, got, low, high)
			}
		}
	})

	t.Run("non-negative", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			got, err := chooseSleepDuration(0, 10*time.Second)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got < 0 {
				t.Errorf("iteration %d: got %v < 0", i, got)
			}
		}
	})
}
