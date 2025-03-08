#!/usr/bin/env python3
"""
Database migration and management utility for Quotron storage
"""

import os
import sys
import argparse
import logging
import psycopg2
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT
from datetime import datetime
import glob

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Database connection parameters
DB_HOST = os.environ.get("DB_HOST", "localhost")
DB_PORT = int(os.environ.get("DB_PORT", "5432"))
DB_NAME = os.environ.get("DB_NAME", "quotron")
DB_USER = os.environ.get("DB_USER", "quotron")
DB_PASS = os.environ.get("DB_PASS", "quotron")

class DBManager:
    """Database migration and management utility"""
    
    def __init__(self, host=DB_HOST, port=DB_PORT, dbname=DB_NAME, user=DB_USER, password=DB_PASS):
        """Initialize the database manager"""
        self.host = host
        self.port = port
        self.dbname = dbname
        self.user = user
        self.password = password
        
        # Base directory of migrations
        self.base_dir = os.path.join(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            "migrations"
        )
        
        self.conn = None
        self.version_table = "schema_migrations"
    
    def connect(self):
        """Connect to the PostgreSQL database"""
        try:
            # Connect to PostgreSQL
            self.conn = psycopg2.connect(
                host=self.host,
                port=self.port,
                dbname=self.dbname,
                user=self.user,
                password=self.password
            )
            self.conn.set_isolation_level(ISOLATION_LEVEL_AUTOCOMMIT)
            logger.info(f"Connected to database {self.dbname}")
        except psycopg2.OperationalError as e:
            logger.error(f"Connection error: {e}")
            sys.exit(1)
    
    def init_version_table(self):
        """Initialize the schema version table if it doesn't exist"""
        with self.conn.cursor() as cur:
            cur.execute(f"""
                CREATE TABLE IF NOT EXISTS {self.version_table} (
                    id SERIAL PRIMARY KEY,
                    version VARCHAR(255) NOT NULL,
                    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
                )
            """)
            logger.info(f"Initialized schema version table: {self.version_table}")
    
    def get_applied_migrations(self):
        """Get a list of migrations that have been applied"""
        with self.conn.cursor() as cur:
            cur.execute(f"SELECT version FROM {self.version_table} ORDER BY id")
            return [row[0] for row in cur.fetchall()]
    
    def get_available_migrations(self):
        """Get a list of available migrations"""
        migration_files = glob.glob(os.path.join(self.base_dir, '*.sql'))
        migrations = []
        
        for file_path in migration_files:
            # Only include up migrations (not down migrations)
            file_name = os.path.basename(file_path)
            if not file_name.endswith('_down.sql'):
                # Extract version number from file name (e.g., 001_initial_schema.sql -> 001)
                version = file_name.split('_')[0]
                migrations.append((version, file_path))
        
        # Sort by version
        migrations.sort(key=lambda x: x[0])
        return migrations
    
    def apply_migration(self, version, file_path):
        """Apply a single migration"""
        with open(file_path, 'r') as f:
            sql = f.read()
        
        with self.conn.cursor() as cur:
            try:
                logger.info(f"Applying migration {version}...")
                cur.execute(sql)
                
                # Record the migration
                cur.execute(
                    f"INSERT INTO {self.version_table} (version) VALUES (%s)",
                    (version,)
                )
                logger.info(f"Successfully applied migration {version}")
                return True
            except Exception as e:
                logger.error(f"Failed to apply migration {version}: {e}")
                self.conn.rollback()
                return False
    
    def migrate_up(self):
        """Apply all pending migrations"""
        self.connect()
        self.init_version_table()
        
        applied = set(self.get_applied_migrations())
        available = self.get_available_migrations()
        
        applied_count = 0
        for version, file_path in available:
            if version not in applied:
                if self.apply_migration(version, file_path):
                    applied_count += 1
        
        if applied_count == 0:
            logger.info("No new migrations to apply")
        else:
            logger.info(f"Applied {applied_count} migrations")
    
    def migrate_down(self, target_version=None):
        """Roll back migrations"""
        self.connect()
        self.init_version_table()
        
        applied = self.get_applied_migrations()
        
        if not applied:
            logger.info("No migrations to roll back")
            return
        
        # Determine which migrations to roll back
        if target_version:
            # Roll back to a specific version
            try:
                target_index = applied.index(target_version)
                versions_to_rollback = applied[target_index + 1:]
            except ValueError:
                logger.error(f"Target version {target_version} not found in applied migrations")
                return
        else:
            # Roll back the most recent migration
            versions_to_rollback = [applied[-1]]
        
        # Roll back migrations in reverse order
        for version in reversed(versions_to_rollback):
            down_file = os.path.join(self.base_dir, f"{version}_down.sql")
            
            if not os.path.exists(down_file):
                logger.error(f"Down migration file not found for version {version}")
                return
            
            with open(down_file, 'r') as f:
                sql = f.read()
            
            with self.conn.cursor() as cur:
                try:
                    logger.info(f"Rolling back migration {version}...")
                    cur.execute(sql)
                    
                    # Remove the migration record
                    cur.execute(
                        f"DELETE FROM {self.version_table} WHERE version = %s",
                        (version,)
                    )
                    logger.info(f"Successfully rolled back migration {version}")
                except Exception as e:
                    logger.error(f"Failed to roll back migration {version}: {e}")
                    self.conn.rollback()
                    return
    
    def status(self):
        """Show migration status"""
        self.connect()
        self.init_version_table()
        
        applied = set(self.get_applied_migrations())
        available = self.get_available_migrations()
        
        logger.info("Migration Status:")
        for version, file_path in available:
            status = "Applied" if version in applied else "Pending"
            logger.info(f"{version}: {os.path.basename(file_path)} - {status}")

def main():
    """Main entry point"""
    parser = argparse.ArgumentParser(description="Database migration utility")
    subparsers = parser.add_subparsers(dest="command", help="Commands")
    
    # Migrate up command
    up_parser = subparsers.add_parser("up", help="Apply pending migrations")
    
    # Migrate down command
    down_parser = subparsers.add_parser("down", help="Roll back migrations")
    down_parser.add_argument("--to", help="Target version to roll back to")
    
    # Status command
    status_parser = subparsers.add_parser("status", help="Show migration status")
    
    args = parser.parse_args()
    
    db_manager = DBManager()
    
    if args.command == "up":
        db_manager.migrate_up()
    elif args.command == "down":
        db_manager.migrate_down(args.to)
    elif args.command == "status":
        db_manager.status()
    else:
        parser.print_help()

if __name__ == "__main__":
    main()