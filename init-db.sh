#!/bin/bash
set -e

# This script runs when PostgreSQL container starts for the first time
echo "Initializing William database..."

# Create database if it doesn't exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'Database william already exists' 
    WHERE EXISTS (SELECT FROM pg_database WHERE datname = 'william')
    UNION ALL
    SELECT 'Creating database william...' 
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'william');
EOSQL

echo "Database initialization completed!" 