#!/bin/bash
set -e

# This script initializes both pllm and dex databases
# It runs automatically when the PostgreSQL container starts for the first time

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    -- Ensure pllm database exists (should already exist from POSTGRES_DB env var)
    SELECT 'Database pllm already exists' WHERE EXISTS (SELECT FROM pg_database WHERE datname = 'pllm');
    
    -- Create dex database for authentication
    CREATE DATABASE dex;
    GRANT ALL PRIVILEGES ON DATABASE dex TO pllm;
    
    -- Connect to dex database and grant schema privileges
    \c dex;
    GRANT ALL ON SCHEMA public TO pllm;
    
    -- Switch back to pllm database
    \c pllm;
    
    -- Ensure pllm user has all privileges on pllm database
    GRANT ALL PRIVILEGES ON DATABASE pllm TO pllm;
EOSQL

echo "Databases initialized successfully!"