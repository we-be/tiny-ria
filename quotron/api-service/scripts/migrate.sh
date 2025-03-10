#!/bin/bash

# Apply database migrations
echo "Applying database migrations..."

# Use environment variables for database connection
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5433}
DB_NAME=${DB_NAME:-quotron}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}

# Create a temporary SQL file with our migration
cat > /tmp/api_service_migration.sql << EOF
$(cat ../storage/migrations/004_api_service_tables.sql)
EOF

# Run the migration
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f /tmp/api_service_migration.sql

# Check if migration was successful
if [ $? -eq 0 ]; then
  echo "✅ Migration applied successfully"
else
  echo "❌ Migration failed"
  exit 1
fi

# Clean up
rm /tmp/api_service_migration.sql