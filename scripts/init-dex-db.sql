-- Create dex database if it doesn't exist
-- This should be run in the postgres container

-- Connect to postgres database first
\c postgres;

-- Create dex database
CREATE DATABASE dex;

-- Grant all privileges to pllm user
GRANT ALL PRIVILEGES ON DATABASE dex TO pllm;

-- Connect to dex database
\c dex;

-- Ensure pllm user has full access
GRANT ALL ON SCHEMA public TO pllm;