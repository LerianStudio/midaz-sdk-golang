# Transaction Ramp-Up Test

A stress testing tool that gradually increases TPS (Transactions Per Second) to find the breaking point of the Midaz transaction API.

## Overview

Unlike a fixed-rate stress test, the ramp-up test starts at a low TPS and incrementally increases until reaching a maximum target. This approach helps identify:

- **Sustainable throughput** - The TPS level the server handles consistently
- **Breaking point** - Where errors start occurring or latency spikes
- **Degradation pattern** - How the server behaves under increasing load

## How It Works

```
TPS
 ▲
 │                          ┌─────────── MAX_TPS (target)
 │                    ┌─────┘
 │              ┌─────┘
 │        ┌─────┘
 │  ┌─────┘
 │──┘ START_TPS
 └──────────────────────────────────────► Time
    │     │     │     │     │
    └──┬──┘     └──┬──┘     │
   STEP_DURATION  STEP_TPS increment
```

## Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

### Required Parameters

| Variable | Description |
|----------|-------------|
| `ORG_ID` | Organization ID |
| `LEDGER_ID` | Ledger ID |
| `FROM_ACCOUNT_ID` | Source account (e.g., `@external/BRL`) |
| `TO_ACCOUNT_ID` | Destination account (e.g., `@destiny`) |

### Test Parameters

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_TPS` | 1000 | Maximum TPS to reach |
| `TX_DURATION` | 60 | Total test duration in seconds |
| `TX_WORKERS` | 100 | Number of parallel workers (goroutines) |
| `TX_AMOUNT` | 1 | Amount per transaction |
| `TX_ASSET` | USD | Asset code |

### Ramp-Up Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RAMP_START_TPS` | 10 | Starting TPS |
| `RAMP_STEP_TPS` | 10 | TPS increment per step |
| `RAMP_STEP_DURATION` | 10 | Seconds between each increment |
| `JITTER_PERCENT` | 20 | Random delay variation (0-100%) |

### Authentication

| Variable | Description |
|----------|-------------|
| `PLUGIN_AUTH_ENABLED` | Enable authentication (`true`/`false`) |
| `PLUGIN_AUTH_ADDRESS` | Auth server URL |
| `MIDAZ_CLIENT_ID` | OAuth client ID |
| `MIDAZ_CLIENT_SECRET` | OAuth client secret |

## Running the Test

```bash
# From repository root
make tx-ramp-test

# Or directly
cd examples/transaction-ramp-test
go run main.go
```

## Understanding Jitter

Jitter adds random variation to request timing to prevent "thundering herd" problems.

### Without Jitter (JITTER_PERCENT=0)

All workers fire at the same instant, causing traffic spikes:

```
Worker 1: ─────●─────●─────●─────●
Worker 2: ─────●─────●─────●─────●
Worker 3: ─────●─────●─────●─────●
               ↑ synchronized spikes
```

### With Jitter (JITTER_PERCENT=20)

Requests are distributed over time:

```
Worker 1: ────●───────●────●──────●
Worker 2: ──────●────●───────●────●
Worker 3: ───●──────●────●───────●
             ↑ distributed load
```

### Jitter Values

| Jitter | Use Case |
|--------|----------|
| 0% | Maximum stress (find absolute limit) |
| 10-20% | Standard load testing |
| 30-40% | High concurrency scenarios |
| 50% | Production traffic simulation |

## Example Configurations

### Quick Validation Test

```env
MAX_TPS=100
TX_DURATION=30
TX_WORKERS=50
RAMP_START_TPS=10
RAMP_STEP_TPS=10
RAMP_STEP_DURATION=5
JITTER_PERCENT=20
```

### Find Breaking Point

```env
MAX_TPS=2000
TX_DURATION=300
TX_WORKERS=200
RAMP_START_TPS=50
RAMP_STEP_TPS=50
RAMP_STEP_DURATION=10
JITTER_PERCENT=0
```

### Production Simulation

```env
MAX_TPS=500
TX_DURATION=600
TX_WORKERS=100
RAMP_START_TPS=20
RAMP_STEP_TPS=20
RAMP_STEP_DURATION=30
JITTER_PERCENT=50
```

## Output

The test displays real-time metrics:

```
[  45s] Success:   1523 | Errors:   12 | Instant TPS:   42 | Avg TPS:  33.8 | Target: 50
>>> TPS increased to 60
[  46s] Success:   1567 | Errors:   12 | Instant TPS:   44 | Avg TPS:  34.1 | Target: 60
```

### Final Report

```
=== Ramp-Up Test Results ===
============================
Max TPS Target:     1000
Actual Avg TPS:     487.32
----------------------------
Total Executed:     29240
Successful:         28891 (98.8%)
Failed:             349 (1.2%)
----------------------------
Duration:           1m0.003s
Avg Latency:        156.42 ms
Min Latency:        45 ms
Max Latency:        2341 ms
============================
```

### Error Log

Errors are saved to `errors_YYYYMMDD_HHMMSS.log` with:
- Error summary by type
- Full list of all errors with transaction index

## Architecture

The test uses `DynamicRateLimiter` from `pkg/concurrent` which provides:

- **Dynamic TPS adjustment** - Rate changes at runtime via atomic operations
- **Jitter support** - Prevents thundering herd with configurable randomness
- **Worker-aware delays** - Calculates per-worker delay to achieve target TPS

### Rate Limiting Formula

```
delay_per_worker = (1 second / target_TPS) * num_workers
```

Example: 100 TPS with 50 workers = 500ms delay per worker

With 20% jitter: delay varies between 400ms and 600ms randomly.

## Tips

1. **Start low** - Begin with conservative values to establish baseline
2. **Increase gradually** - Use small step increments for better granularity
3. **Watch for errors** - First errors often indicate the sustainable limit
4. **Monitor latency** - Latency spikes precede error increases
5. **Use jitter** - Prevents artificial synchronization issues
6. **Long duration** - Allow time for the system to stabilize at each level
