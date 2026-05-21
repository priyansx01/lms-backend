#!/bin/bash
set -a
source .env
set +a
go run cmd/api/main.go > api.log 2>&1
