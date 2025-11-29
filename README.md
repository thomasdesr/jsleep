# jsleep

A sleep command with built-in jitter. Useful for avoiding thundering herd problems in cron jobs, distributed systems, and anywhere you want randomized delays.

## Installation

```bash
go install github.com/thomasdesr/jsleep@latest
```

## Usage

```bash
# Sleep ~10s with default ±50% jitter (5s-15s)
jsleep 10s

# Sleep ~10s with ±20% jitter (8s-12s)
jsleep 10s 20%
jsleep 10s --jitter 20%

# Sleep ~10s with ±2s absolute jitter (8s-12s)
jsleep 10s --range 2s

# Bound the jitter: sleep ~10s but never less than 9s (9s-15s)
jsleep --min 9s 10s

# Bound both ends: sleep ~10s but clamp to 8s-11s
jsleep --min 8s --max 11s 10s

# Just specify bounds directly (no base duration)
jsleep --min 5s --max 15s

# See the chosen duration
jsleep -v 10s
```

## Options

| Flag | Description |
|------|-------------|
| `-j, --jitter <percent>` | Jitter as percent (default: 50%) |
| `-r, --range <duration>` | Absolute jitter range (±duration) |
| `-m, --min <duration>` | Clamp jitter result to this minimum |
| `-M, --max <duration>` | Clamp jitter result to this maximum |
| `-v, --verbose` | Print chosen duration to stderr |

## Duration Format

Supports standard Go duration units (`ms`, `s`, `m`, `h`) plus days (`d`). Bare numbers default to seconds.

```bash
jsleep 100      # 100 seconds
jsleep 1.5h     # 1 hour 30 minutes
jsleep 2d       # 2 days
```

## Examples

```bash
# Cron job with jitter to spread load
*/5 * * * * jsleep 2m && /usr/local/bin/my-task

# Retry with randomized backoff
jsleep --min 1s --max 30s && retry-command

# Fixed delay with small variance
jsleep 60s --range 5s
```
