#!/bin/bash
set -a
source .env
set +a
go run cmd/worker/main.go > worker.log 2>&1
