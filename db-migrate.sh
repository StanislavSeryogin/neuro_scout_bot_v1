#!/bin/sh

$HOME/go/bin/goose -dir ./internal/storage/migrations postgres "host=localhost user=DBUSER dbname=neuro_scout_bot_v1 password=DBPASSWORD sslmode=disable" "$@"
# Replace DBUSER, DBNAME, and DBPASSWORD with your actual credentials 