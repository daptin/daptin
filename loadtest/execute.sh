#!/usr/bin/env bash
vegeta attack -rate=$3/1s -targets=$1/$2/attack.txt -body=$1/$2/postbody.json -duration=${4}s | tee results.bin | vegeta report
