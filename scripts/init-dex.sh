#!/bin/bash

# Initialize Dex database in the shared PostgreSQL container

echo "Initializing Dex database..."

# Wait for PostgreSQL to be ready
until docker exec pllm-postgres pg_isready -U pllm; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

# Create dex database
docker exec -i pllm-postgres psql -U pllm -d postgres <<EOF
-- Create dex database if it doesn't exist
SELECT 'CREATE DATABASE dex'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'dex')\gexec

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE dex TO pllm;
EOF

echo "Dex database initialized successfully!"