package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const defaultJitterFraction = 0.5

func main() {
	low, high, verbose, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsleep: %v\n", err)
		os.Exit(1)
	}

	sleepValue, err := chooseSleepDuration(low, high)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsleep: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "sleeping for %s\n", sleepValue.Round(time.Millisecond))
	}

	time.Sleep(sleepValue)
}

func usage() {
	fmt.Fprint(os.Stderr, `jsleep - jittered sleep

Usage:
  jsleep <duration>                    Default 50% jitter
  jsleep <duration> <percent>          Positional percent jitter (e.g., 25%)
  jsleep <duration> --jitter <percent> Explicit percent jitter
  jsleep <duration> --range <duration> Absolute jitter range (±duration)
  jsleep --min <duration> --max <duration>

Options:
  -j, --jitter <percent>   Jitter as percent (e.g., 20%); defaults to 50%.
  -r, --range <duration>   Absolute jitter range (e.g., 2s for +/- 2 seconds).

  -m, --min <duration>     Clamp jitter result to this minimum (e.g., jsleep --min 9s 10s).
  -M, --max <duration>     Clamp jitter result to this maximum.

  -v, --verbose            Print the chosen sleep duration to stderr.
  -h, --help               Show this help.
`)
}

func parseArgs(args []string) (low, high time.Duration, verbose bool, err error) {
	fs := flag.NewFlagSet("jsleep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = usage

	var jitterStr, rangeStr, minStr, maxStr string
	fs.StringVar(&jitterStr, "jitter", "", "percent jitter (e.g., 20%)")
	fs.StringVar(&jitterStr, "j", "", "percent jitter (e.g., 20%)")
	fs.StringVar(&rangeStr, "range", "", "absolute jitter range (e.g., 2s for ±2 seconds)")
	fs.StringVar(&rangeStr, "r", "", "absolute jitter range (e.g., 2s for ±2 seconds)")
	fs.StringVar(&minStr, "min", "", "minimum duration bound")
	fs.StringVar(&minStr, "m", "", "minimum duration bound")
	fs.StringVar(&maxStr, "max", "", "maximum duration bound")
	fs.StringVar(&maxStr, "M", "", "maximum duration bound")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&verbose, "v", false, "verbose output")

	if err = fs.Parse(args); err != nil {
		return
	}

	pos := fs.Args()
	if len(pos) > 2 {
		err = errors.New("too many positional arguments")
		return
	}

	var positionalJitter string
	if len(pos) == 2 {
		positionalJitter = pos[1]
	}

	jitterSet := jitterStr != ""
	rangeSet := rangeStr != ""
	minSet := minStr != ""
	maxSet := maxStr != ""

	if jitterSet && rangeSet {
		err = errors.New("cannot use --jitter with --range")
		return
	}
	if positionalJitter != "" && jitterSet {
		err = errors.New("cannot specify jitter both positionally and with --jitter")
		return
	}
	if positionalJitter != "" && rangeSet {
		err = errors.New("cannot use positional jitter with --range")
		return
	}

	var rangeVal, minVal, maxVal time.Duration
	if rangeSet {
		if rangeVal, err = parseDuration(rangeStr); err != nil {
			return
		}
	}
	if minSet {
		if minVal, err = parseDuration(minStr); err != nil {
			return
		}
	}
	if maxSet {
		if maxVal, err = parseDuration(maxStr); err != nil {
			return
		}
	}
	if minSet && maxSet && maxVal < minVal {
		err = errors.New("max must be greater than or equal to min")
		return
	}

	var base time.Duration
	var hasBase bool
	if len(pos) == 1 || len(pos) == 2 {
		if base, err = parseDuration(pos[0]); err != nil {
			return
		}
		hasBase = true
	}

	switch {
	case rangeSet:
		if !hasBase {
			err = errors.New("--range requires a base duration")
			return
		}
		low, high = base-rangeVal, base+rangeVal

	case hasBase:
		fraction := defaultJitterFraction
		if jitterSet {
			if fraction, err = parsePercent(jitterStr); err != nil {
				return
			}
		} else if positionalJitter != "" {
			if fraction, err = parsePercent(positionalJitter); err != nil {
				return
			}
		}
		baseNs := float64(base.Nanoseconds())
		delta := math.Round(baseNs * fraction)
		if math.IsNaN(delta) || math.IsInf(delta, 0) {
			err = errors.New("jitter results overflow time.Duration")
			return
		}
		lowNs, highNs := baseNs-delta, baseNs+delta
		if lowNs < math.MinInt64 || lowNs > math.MaxInt64 || highNs < math.MinInt64 || highNs > math.MaxInt64 {
			err = errors.New("jitter results overflow time.Duration")
			return
		}
		low, high = time.Duration(lowNs), time.Duration(highNs)

	case minSet && maxSet:
		low, high = minVal, maxVal

	default:
		err = errors.New("missing required duration")
		return
	}

	if minSet {
		low, high = max(low, minVal), max(high, minVal)
	}
	if maxSet {
		low, high = min(low, maxVal), min(high, maxVal)
	}
	low, high = max(low, 0), max(high, 0)

	if high < low {
		err = errors.New("defined interval is empty after clamping")
		return
	}
	return
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}

	// Handle days.
	if strings.HasSuffix(s, "d") {
		num, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}

		const maxDays = float64(math.MaxInt64) / float64(24*time.Hour)
		if num > maxDays || num < -maxDays {
			return 0, fmt.Errorf("duration out of range: %s", s)
		}

		return time.Duration(num * float64(24*time.Hour)), nil
	}

	// Append "s" if the duration is a number without a unit.
	if unicode.IsDigit(rune(s[len(s)-1])) {
		s += "s"
	}
	return time.ParseDuration(s)
}

func parsePercent(s string) (float64, error) {
	if !strings.HasSuffix(s, "%") {
		return 0, fmt.Errorf("percent must end with %%: %s", s)
	}
	val, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid percent: %s", s)
	}
	if val < 0 {
		return 0, errors.New("jitter cannot be negative")
	}
	return val / 100, nil
}

func chooseSleepDuration(low, high time.Duration) (time.Duration, error) {
	if high == low {
		return max(low, 0), nil
	}

	if low > high {
		return 0, errors.New("low must be less than or equal to high")
	}

	width := high - low
	if low+width == math.MaxInt64 {
		return high, nil
	}

	offset, err := cryptoRandInt64(int64(width) + 1)
	if err != nil {
		return 0, err
	}
	return max(low+time.Duration(offset), 0), nil
}

func cryptoRandInt64(n int64) (int64, error) {
	if n <= 0 {
		return 0, errors.New("n must be positive")
	}

	var buf [8]byte
	maxUint := ^uint64(0)
	limit := maxUint - (maxUint % uint64(n))

	for range 1000 {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0, err
		}
		v := binary.LittleEndian.Uint64(buf[:])
		if v < limit {
			return int64(v % uint64(n)), nil
		}
	}

	return 0, errors.New("random number generation failed after too many attempts")
}
