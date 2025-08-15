#!/bin/bash

# Configuration variables - edit these as needed
PROJECT_NAME="karthik-test"
BATCH_NAME="respectable-gray-moth"

# Supervise command variables
MAX_RERUN_ATTEMPTS=2
MAX_FAILED_JOB_THRESHOLD=100
RERUN_ON_STATES="Error, Blocker"

# Add your test commands here
echo "Running resim command..."

# Example commands you might want to test:
./resim batches supervise --project "$PROJECT_NAME" --batch-name "$BATCH_NAME" --max-rerun-attempts "$MAX_RERUN_ATTEMPTS" --max-failed-job-threshold "$MAX_FAILED_JOB_THRESHOLD" --rerun-on-states "$RERUN_ON_STATES"

# Wait batch command
# ./resim batches tests --project "$PROJECT_NAME" --batch-name "$BATCH_NAME"

# Or test other commands:
# ./resim batches create --project "$PROJECT_NAME" --build-id "build-uuid" --experiences "test-exp"
# ./resim systems create --project "$PROJECT_NAME" --name "test-system" --description "test" --build-vcpus 4
