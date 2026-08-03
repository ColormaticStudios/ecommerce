#!/usr/bin/env bash
set -e


wait_for_dev_db() {
	local container="ecommerce-db"
	local deadline=$((SECONDS + 60))
	local state

	while ((SECONDS < deadline)); do
		state="$(sudo docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container" 2>/dev/null || true)"

		case "$state" in
			"running healthy")
				# Docker health checks run inside the container. Confirm that the
				# host connection used by migrations is also ready before proceeding.
				if go run ./cmd/migrate status >/dev/null 2>&1; then
					return 0
				fi
				;;
			exited* | dead*)
				echo "Dev DB container stopped before becoming ready" >&2
				return 1
				;;
		esac

		sleep 0.25
	done

	echo "Dev DB did not become ready within 60 seconds" >&2
	return 1
}


echo "Starting Docker Daemon..."
sudo systemctl start docker

echo "Starting dev DB..."
tmux new-session -d -s "ecommerce-dev" -n "database" 'sudo scripts/run-dev-db-docker.sh'

wait_for_dev_db

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
