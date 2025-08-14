-- Migration: Add DexID column to users table
-- This migration adds support for Dex OAuth authentication

-- Add dex_id column as nullable first
ALTER TABLE users ADD COLUMN IF NOT EXISTS dex_id TEXT;

-- Create unique index on dex_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_dex_id ON users(dex_id) WHERE dex_id IS NOT NULL;

-- For existing users, we'll generate a placeholder dex_id based on their email
-- This allows them to continue using the system until they authenticate via Dex
UPDATE users 
SET dex_id = 'legacy:' || email 
WHERE dex_id IS NULL;

-- Now that all rows have a value, we can make it NOT NULL if needed
-- But we'll keep it nullable to allow for future flexibility
-- ALTER TABLE users ALTER COLUMN dex_id SET NOT NULL;