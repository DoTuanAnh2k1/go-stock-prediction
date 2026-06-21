#!/usr/bin/env bash
# ===========================================================================
# svc-metrics.sh — per-service resource snapshot for the go-stock-prediction
#                  docker-compose stack.
# ---------------------------------------------------------------------------
# Usage:
#   bash scripts/svc-metrics.sh            # one-shot snapshot
#   bash scripts/svc-metrics.sh --watch N  # refresh every N seconds (log-safe loop)
#   bash scripts/svc-metrics.sh --json     # NDJSON lines (one JSON object/container)
#   bash scripts/svc-metrics.sh -h|--help  # this message
#
# Output columns (table mode):
#   CONTAINER  CPU%  MEM_USAGE  MEM_LIMIT  MEM%  NET_IO  BLOCK_IO  PIDS
# Rows are sorted by MEM_USAGE descending. A TOTAL line sums CPU% and MEM.
#
# Requirements: bash 4+, docker CLI in PATH.  No extra dependencies.
# ===========================================================================
set -euo pipefail

# ---------------------------------------------------------------------------
# Known container names for this compose stack (order = display priority when
# all are running; the running subset is detected automatically).
# ---------------------------------------------------------------------------
ALL_KNOWN=(
  timescaledb
  service-mgt
  auth-svc
  prediction-svc
  api-svc
  gateway-svc
  web-svc
  cli-svc
  pgadmin
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
usage() {
  sed -n '2,12p' "$0" | sed 's/^# \?//'
  exit 0
}

die() { echo "ERROR: $*" >&2; exit 1; }

# Emit a newline-separated list of known containers that are currently running.
running_containers() {
  local running
  running=$(docker ps --format '{{.Names}}' 2>/dev/null) || die "docker ps failed"
  local name
  for name in "${ALL_KNOWN[@]}"; do
    if echo "$running" | grep -qx "$name"; then
      echo "$name"
    fi
  done
}

# ---------------------------------------------------------------------------
# Core: collect stats, sort by memory descending, print table + TOTAL line.
# ---------------------------------------------------------------------------
print_table() {
  local -a containers=("$@")
  if [[ ${#containers[@]} -eq 0 ]]; then
    echo "(no matching containers running)"
    return
  fi

  # Collect raw stats: Name CPU MemUsage/Limit MemPerc NetIO BlockIO PIDs
  local fmt='{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}'
  local raw
  raw=$(docker stats --no-stream --format "$fmt" "${containers[@]}" 2>/dev/null) || true

  if [[ -z "$raw" ]]; then
    echo "(docker stats returned no data)"
    return
  fi

  # Hand the raw TSV to awk which:
  #  1. Parses each line, converts mem_usage to MiB for sorting
  #  2. Collects all rows into arrays
  #  3. Sorts by mem_mib descending (bubble sort — N≤9)
  #  4. Prints header, rows, separator, TOTAL
  echo "$raw" | awk -F'\t' '
  # -------------------------------------------------------------------------
  # convert "1.23GiB", "512MiB", "768KiB", "999B" → float MiB
  function to_mib(s,   val) {
    val = s
    if      (val ~ /GiB$/) { sub(/GiB$/, "", val); return val * 1024 }
    else if (val ~ /MiB$/) { sub(/MiB$/, "", val); return val + 0 }
    else if (val ~ /KiB$/) { sub(/KiB$/, "", val); return val / 1024 }
    else if (val ~ /GB$/)  { sub(/GB$/,  "", val); return val * 953.674 }
    else if (val ~ /MB$/)  { sub(/MB$/,  "", val); return val * 0.953674 }
    else if (val ~ /KB$/)  { sub(/KB$/,  "", val); return val / 1048.576 }
    else                   { sub(/B$/,   "", val); return val / 1048576 }
  }

  # format MiB back to human-readable
  function from_mib(m) {
    if      (m >= 1024) return sprintf("%.2fGiB", m / 1024)
    else if (m >= 1)    return sprintf("%.2fMiB", m)
    else                return sprintf("%.3fKiB", m * 1024)
  }

  # -------------------------------------------------------------------------
  {
    n++
    name[n]    = $1
    cpu[n]     = $2
    memraw[n]  = $3              # "X / Y"
    mempct[n]  = $4
    netio[n]   = $5
    blockio[n] = $6
    pids[n]    = $7

    # split mem usage / limit
    split($3, a, " / ")
    memusage[n] = a[1]
    memlimit[n] = a[2]
    memmib[n]   = to_mib(a[1])

    # accumulate totals
    cpuval = $2; sub(/%$/, "", cpuval)
    total_cpu += cpuval
    total_mem += memmib[n]
  }

  END {
    # ---- bubble sort by memmib descending ---------------------------------
    do {
      swapped = 0
      for (i = 1; i < n; i++) {
        if (memmib[i] < memmib[i+1]) {
          # swap all parallel arrays
          tmp = memmib[i];    memmib[i]    = memmib[i+1];    memmib[i+1]    = tmp
          tmp = name[i];      name[i]      = name[i+1];      name[i+1]      = tmp
          tmp = cpu[i];       cpu[i]       = cpu[i+1];       cpu[i+1]       = tmp
          tmp = memusage[i];  memusage[i]  = memusage[i+1];  memusage[i+1]  = tmp
          tmp = memlimit[i];  memlimit[i]  = memlimit[i+1];  memlimit[i+1]  = tmp
          tmp = mempct[i];    mempct[i]    = mempct[i+1];    mempct[i+1]    = tmp
          tmp = netio[i];     netio[i]     = netio[i+1];     netio[i+1]     = tmp
          tmp = blockio[i];   blockio[i]   = blockio[i+1];   blockio[i+1]   = tmp
          tmp = pids[i];      pids[i]      = pids[i+1];      pids[i+1]      = tmp
          swapped = 1
        }
      }
    } while (swapped)

    # ---- header -----------------------------------------------------------
    sep = sprintf("%110s", ""); gsub(/ /, "-", sep)
    printf "\n"
    printf "%-20s  %7s  %12s  %12s  %7s  %20s  %20s  %6s\n",
      "CONTAINER", "CPU%", "MEM_USAGE", "MEM_LIMIT", "MEM%", "NET_IO", "BLOCK_IO", "PIDS"
    print sep

    # ---- rows -------------------------------------------------------------
    for (i = 1; i <= n; i++) {
      printf "%-20s  %7s  %12s  %12s  %7s  %20s  %20s  %6s\n",
        name[i], cpu[i], memusage[i], memlimit[i], mempct[i], netio[i], blockio[i], pids[i]
    }

    # ---- TOTAL ------------------------------------------------------------
    print sep
    printf "%-20s  %7s  %12s\n",
      "TOTAL (" n " svc)", sprintf("%.2f%%", total_cpu), from_mib(total_mem)
    printf "\n"
  }
  '
}

# ---------------------------------------------------------------------------
# JSON mode: emit raw docker stats NDJSON, one object per running container.
# ---------------------------------------------------------------------------
print_json() {
  local -a containers=("$@")
  if [[ ${#containers[@]} -eq 0 ]]; then
    echo '{"error":"no matching containers running"}'
    return
  fi
  docker stats --no-stream --format '{{json .}}' "${containers[@]}" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
MODE="table"
WATCH_SEC=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)  usage ;;
    --json)     MODE="json"; shift ;;
    --watch)
      [[ $# -lt 2 ]] && die "--watch requires a numeric argument (seconds)"
      [[ "$2" =~ ^[0-9]+$ ]] || die "--watch argument must be a positive integer, got: $2"
      WATCH_SEC="$2"; shift 2
      ;;
    *) die "Unknown argument: $1 (try --help)" ;;
  esac
done

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main_once() {
  # Build array from newline-separated output
  local -a containers
  mapfile -t containers < <(running_containers)

  if [[ "$MODE" == "json" ]]; then
    print_json "${containers[@]:-}"
  else
    local ts
    ts=$(date '+%Y-%m-%d %H:%M:%S')
    echo "=== go-stock-prediction resource snapshot @ ${ts} ==="
    print_table "${containers[@]:-}"
  fi
}

if [[ -n "$WATCH_SEC" ]]; then
  while true; do
    main_once
    sleep "$WATCH_SEC"
  done
else
  main_once
fi
