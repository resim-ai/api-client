#!/bin/bash

# Configuration variables - edit these as needed
PROJECT_NAME="karthik-test"
BATCH_NAME="reanimated-poppy-ox"

# Supervise command variables
MAX_RERUN_ATTEMPTS=3
MAX_FAILED_JOB_THRESHOLD=25
RERUN_ON_STATES="Warning,Error"

# Add your test commands here
echo "Running resim command..."

# Example commands you might want to test:
./resim batches supervise --project "$PROJECT_NAME" --batch-name "$BATCH_NAME" --max-rerun-attempts "$MAX_RERUN_ATTEMPTS" --max-failed-job-threshold "$MAX_FAILED_JOB_THRESHOLD" --rerun-on-states "$RERUN_ON_STATES"

# Or test other commands:
# ./resim batches create --project "$PROJECT_NAME" --build-id "build-uuid" --experiences "test-exp"
# ./resim systems create --project "$PROJECT_NAME" --name "test-system" --description "test" --build-vcpus 4
