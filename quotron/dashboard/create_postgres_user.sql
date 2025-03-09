-- Create or update the postgres user with password 'postgres'
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'postgres') THEN
        ALTER USER postgres WITH PASSWORD 'postgres';
    ELSE
        CREATE USER postgres WITH PASSWORD 'postgres' SUPERUSER;
    END IF;
END
$$;