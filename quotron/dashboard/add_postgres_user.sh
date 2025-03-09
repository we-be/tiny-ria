#!/bin/bash

echo "Attempting to create/update postgres user..."
echo "This script requires sudo access to run commands as the postgres system user."

sudo -u postgres psql -f create_postgres_user.sql

if [ $? -eq 0 ]; then
    echo "Successfully created/updated postgres user with password 'postgres'"
    
    # Test the new user
    PGPASSWORD=postgres psql -U postgres -h localhost -d quotron -c "SELECT current_user;" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "Successfully connected with postgres user"
    else
        echo "Created user but connection test failed. You may need to check pg_hba.conf settings."
    fi
else
    echo "Failed to create/update postgres user. Please run this command manually:"
    echo "sudo -u postgres psql -c \"ALTER USER postgres WITH PASSWORD 'postgres';\""
fi