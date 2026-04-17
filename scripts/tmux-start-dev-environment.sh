#!/usr/bin/env bash
set -e

echo "Starting Docker Daemon..."
sudo systemctl start docker

echo "Starting dev DB..."
tmux new-session -d -s "ecommerce-dev" -n "database" 'sudo scripts/run-dev-db-docker.sh'

# Wait for Postgres to initialize (technically a race condition, but easier this way)
sleep 5

echo "Running database migrations..."
make migrate

echo "Populating dev database..."
scripts/populate-test-database.sh

echo "Starting backend..."
tmux new-window -d -a -t "ecommerce-dev:database" -n "backend" 'make run'
echo "Starting frontend..."
tmux new-window -d -a -t "ecommerce-dev:backend" -n "frontend" 'cd frontend && bun dev'
echo "Starting pages..."
tmux new-window -d -a -t "ecommerce-dev:frontend" -n "pages" 'cd frontend && bun run storybook --no-open'

tmux a -t "ecommerce-dev:frontend"
