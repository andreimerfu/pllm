-- Migration: Remove password column from users table
-- Since Dex handles authentication, we don't need to store passwords

-- Drop the password column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS password;

-- Add avatar_url column if it doesn't exist
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;

-- Ensure external_provider column exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS external_provider VARCHAR(50);

-- Update any NULL external_provider values to 'local' for existing users
UPDATE users SET external_provider = 'local' WHERE external_provider IS NULL;