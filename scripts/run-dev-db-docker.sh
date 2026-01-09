#!/usr/bin/env bash

# Set the database URL in .env to postgresql://ecommerce:123@localhost:5432/ecommerce

docker run --rm --name=ecommerce-db -p 5432:5432 -e POSTGRES_USER=ecommerce -e POSTGRES_DB=ecommerce -e POSTGRES_PASSWORD=123 docker.io/postgres