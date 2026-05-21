#!/usr/bin/env bash
set -e


wait_for_dev_db() {
	local container="ecommerce-db"
	local state

	while true; do
		state="$(sudo docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container" 2>/dev/null || true)"

		case "$state" in
			"running healthy")
				return 0
				;;
			exited* | dead*)
				echo "Dev DB container stopped before becoming healthy" >&2
				return 1
				;;
		esac

		sleep 0.25
	done
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
