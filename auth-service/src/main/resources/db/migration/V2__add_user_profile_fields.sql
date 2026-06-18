-- Flyway V2: add profile fields to users table
-- PostgreSQL supports ADD COLUMN IF NOT EXISTS natively

ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email     VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone     VARCHAR(30);
