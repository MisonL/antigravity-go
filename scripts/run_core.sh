#!/bin/bash
BIN=$1
GEMINI_DIR=$2
APP_DATA_DIR=$3

# Use the verified manual test logic
(printf "\x0a\x02{}"; while true; do sleep 1; done) | "$BIN" \
  --enable_lsp \
  --gemini_dir "$GEMINI_DIR" \
  --app_data_dir "$APP_DATA_DIR" \
  --random_port=true \
  --logtostderr=true
