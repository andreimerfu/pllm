-- Create dex database for authentication
-- This script runs automatically when PostgreSQL container starts

-- Create the dex database if it doesn't exist
SELECT 'CREATE DATABASE dex'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'dex')\gexec

-- Connect to the dex database
\c dex;

-- Grant all privileges to pllm user
GRANT ALL PRIVILEGES ON DATABASE dex TO pllm;
GRANT ALL ON SCHEMA public TO pllm;