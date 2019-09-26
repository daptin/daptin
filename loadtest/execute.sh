#!/usr/bin/env bash
vegeta attack -rate=$2/1s -targets=$1/attack.txt -body=$1/postbody.json -duration=60s | tee results.bin | vegeta report
