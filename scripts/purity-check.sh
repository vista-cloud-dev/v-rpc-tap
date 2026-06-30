#!/usr/bin/env bash
# D10 dependency-purity gate (proposal §6) — RED-GATE.
# The three in-engine VSLRT* routines must reference NO STD*/external-VSL* library
# and do ZERO crypto/hash/encryption/compression/JSON/base64/HTTP/S3/SigV4/network.
# Permitted external symbols are ONLY: the XWBPRS splice reads, ^XTMP storage, the
# per-engine liveness primitives ($ZGETJPI/^$JOB), and — reaper-only — Kernel
# TaskMan (^%ZTLOAD). Adding any other dependency is a design violation (D10), not
# an enhancement. Run from the repo root (or via `make m-purity`).
#
# CODE ONLY: each line is stripped of its M comment (`;` to EOL) before matching,
# so prose like "the live client socket" never trips the gate. (Safe here — no
# VSLRT* string literal contains a ";".)
set -euo pipefail
cd "$(dirname "$0")/.."

SRC=(src/VSLRTAP.m src/VSLRTH.m src/VSLRTRP.m)
OWN='VSLRTAP|VSLRTH|VSLRTRP'   # the package's own three routines — intra-package refs are allowed
fail=0

# scan <regex> <message>: grep CODE (comment-stripped) across the three routines.
scan() {
  local pat="$1" msg="$2" hit="" f out
  for f in "${SRC[@]}"; do
    out=$(sed 's/;.*//' "$f" | grep -nE "$pat" || true)
    [ -n "$out" ] && hit+=$(printf '%s\n' "$out" | sed "s|^|$f:|")$'\n'
  done
  if [ -n "$hit" ]; then printf '%s' "$hit"; echo "PURITY FAIL: $msg"; fail=1; fi
}

# 1. No STD* (m-stdlib) references.
scan '\^%?STD[A-Z]' "STD* (m-stdlib) reference — the tap must be self-contained (D10)"

# 2. No VSL* references other than the package's own three routines.
#    (filter the allowed own-routine refs out of the match set)
own_hits=""
for f in "${SRC[@]}"; do
  o=$(sed 's/;.*//' "$f" | grep -nE '\^VSL[A-Z]' | grep -vE "\^(${OWN})\b" || true)
  [ -n "$o" ] && own_hits+=$(printf '%s\n' "$o" | sed "s|^|$f:|")$'\n'
done
if [ -n "$own_hits" ]; then printf '%s' "$own_hits"; echo "PURITY FAIL: external VSL* reference — only VSLRTAP/VSLRTH/VSLRTRP allowed (D10)"; fail=1; fi

# 3. No crypto / hash / encryption / compression / JSON / base64 / HTTP / S3 / SigV4 / raw network.
scan 'STDCRYPTO|STDJSON|STDB64|STDHEX|STDURL|STDHTTP|STDCOMPRESS|sigv4|encrypt|decrypt|\$ZF\(|\^%ZISH|\^%ZISTCP|LISTEN\^' \
     "forbidden op (crypto/json/base64/http/s3/sigv4/compression/network) — all host-side (D10)"

if [ "$fail" -eq 0 ]; then
  echo "PURITY OK: VSLRTAP/VSLRTH/VSLRTRP reference no STD*/external-VSL*, no crypto/json/net (D10)"
fi
exit "$fail"
