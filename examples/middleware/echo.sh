#!/usr/bin/env bash
#
# `xxd` utility included into vim-common package
# It allow hex decoding/encoding
# 
# This example may broke if you request contains `null` string, you may consider using pipes instead.
# See: https://github.com/buger/gor/issues/309
# 

function log {
    if [[ ! -v GOR_TEST ]]; then # if we are not testing
    # Logging to stderr, because stdout/stdin used for data transfer
    >&2 echo "[DEBUG][ECHO] $1"
    fi
}

while read line; do
    decoded=$(echo -e "$line" | xxd -r -p)

    header=$(echo -e "$decoded" | head -n +1)
    payload=$(echo -e "$decoded" | tail -n +2)

    encoded=$(echo -e "$header\n$payload" | xxd -p | tr -d "\\n")

    log ""
    log "==================================="

    case ${header:0:1} in
    "1")
        log "Request type: Request"
        ;;
    "2")
        log "Request type: Original Response"
        ;;
    "3")
        log "Request type: Replayed Response"
        ;;
    *)
        log "Unknown request type $header"
    esac
    echo "$encoded"

    log "==================================="

    log "Original data: $line"
    log "Decoded request: $decoded"
    log "Encoded data: $encoded"
done;
