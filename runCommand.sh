#!/bin/bash

PROJECT_NAME="karthik-test"
BATCH_NAME="repressive-indigo-muskox"

# Supervise command variables
MAX_RERUN_ATTEMPTS=1
RERUN_MAX_FAILURE_PERCENT=100
RERUN_ON_STATES="Error,Warning,Blocker"

# Add your test commands here
echo "Running resim command..."

# Example commands you might want to test:
# ./resim batches supervise --project "$PROJECT_NAME" --batch-name "$BATCH_NAME" --max-rerun-attempts "$MAX_RERUN_ATTEMPTS" --rerun-max-failure-percent "$RERUN_MAX_FAILURE_PERCENT" --rerun-on-states "$RERUN_ON_STATES"

# Wait for batch completion
./resim batches wait --project "$PROJECT_NAME" --batch-name "$BATCH_NAME" --wait-timeout "20s"

