#!/bin/bash
# Generate a small synthetic mbox file with CRLF line endings and unique Message-IDs
# Used as fallback when dovecot-crlf download fails
# Output: /tmp/dovecot-crlf

OUTFILE="${1:-/tmp/dovecot-crlf}"

generate_mbox() {
    local out="$OUTFILE"
    > "$out"

    for i in $(seq 1 5); do
        local msgid="test-$(date +%s)-${i}-$$@localhost"
        local date
        date=$(date -R 2>/dev/null || date "+%a, %d %b %Y %H:%M:%S %z")

        # Write mbox separator and headers with CRLF
        printf "From sender${i}@example.com %s\r\n" "$(date '+%a %b %d %H:%M:%S %Y')" >> "$out"
        printf "From: sender${i}@example.com\r\n" >> "$out"
        printf "To: testuser@localhost\r\n" >> "$out"
        printf "Subject: Test message ${i}\r\n" >> "$out"
        printf "Date: %s\r\n" "$date" >> "$out"
        printf "Message-ID: <%s>\r\n" "$msgid" >> "$out"
        printf "MIME-Version: 1.0\r\n" >> "$out"
        printf "Content-Type: text/plain; charset=UTF-8\r\n" >> "$out"
        printf "\r\n" >> "$out"
        printf "This is test message number ${i} for IMAP testing.\r\n" >> "$out"
        printf "It contains enough text to be a valid email body.\r\n" >> "$out"
        printf "\r\n" >> "$out"
    done

    echo "Generated $OUTFILE with 5 messages"
}

generate_mbox
