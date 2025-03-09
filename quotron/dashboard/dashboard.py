import streamlit as st
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
import requests
import subprocess
import os
import json
import time
from datetime import datetime, timedelta
import psycopg2
from psycopg2.extras import RealDictCursor
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Set default environment variables to the known working configuration
if "DB_HOST" not in os.environ:
    os.environ["DB_HOST"] = "localhost"
if "DB_PORT" not in os.environ:
    os.environ["DB_PORT"] = "5432"  # Default PostgreSQL port
if "DB_NAME" not in os.environ:
    os.environ["DB_NAME"] = "quotron"  # Database name
if "DB_USER" not in os.environ:
    os.environ["DB_USER"] = "quotron"  # ALWAYS use quotron as default
if "DB_PASSWORD" not in os.environ:
    os.environ["DB_PASSWORD"] = "quotron"  # ALWAYS use quotron as default

# Database connection
def get_db_connection():
    """Create a connection to the PostgreSQL database."""
    # No try-except to let errors propagate up
    conn = psycopg2.connect(
        host=os.environ["DB_HOST"],
        port=os.environ["DB_PORT"],
        database=os.environ["DB_NAME"],
        user=os.environ["DB_USER"],
        password=os.environ["DB_PASSWORD"]
    )
    return conn

# Scheduler control
SCHEDULER_LOG_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "scheduler.log")

def get_scheduler_status():
    """Check if the scheduler is running.
    
    This is a simplified, reliable implementation that just checks for the actual running Go process.
    """
    try:
        # Look specifically for the Go scheduler process, not our wrapper script
        result = subprocess.run(
            "ps aux | grep '[g]o run cmd/scheduler/main.go'", 
            shell=True, 
            capture_output=True, 
            text=True
        )
        return len(result.stdout.strip()) > 0
    except Exception as e:
        log_message(f"Error checking scheduler status: {e}")
        return False

def log_message(message):
    """Log a message to the scheduler log file."""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    # Filter out any potential API keys before logging
    # This is a basic approach - in production you'd want more sophisticated patterns
    filtered_message = message
    api_key_patterns = [
        os.environ.get("ALPHA_VANTAGE_API_KEY", ""),
        os.environ.get("FINANCE_API_KEY", ""),
        "api_key=", "apikey=", "key=", "token="
    ]
    
    # Remove non-empty patterns
    for pattern in api_key_patterns:
        if pattern and len(pattern) > 5:  # Only filter non-empty meaningful patterns
            filtered_message = filtered_message.replace(pattern, "[API_KEY_REDACTED]")
    
    log_entry = f"[{timestamp}] {filtered_message}\n"
    
    try:
        with open(SCHEDULER_LOG_FILE, 'a') as f:
            f.write(log_entry)
    except Exception as e:
        st.error(f"Error writing to log: {e}")

def get_scheduler_logs(lines=50):
    """Get the last n lines from the log file."""
    if not os.path.exists(SCHEDULER_LOG_FILE):
        return ["No logs available yet."]
    
    try:
        with open(SCHEDULER_LOG_FILE, 'r') as f:
            logs = f.readlines()
        return logs[-lines:] if logs else ["No log entries found."]
    except Exception as e:
        return [f"Error reading logs: {e}"]

def clear_logs():
    """Clear the scheduler log file."""
    try:
        with open(SCHEDULER_LOG_FILE, 'w') as f:
            f.write("")
        log_message("Logs cleared")
        return True
    except Exception as e:
        st.error(f"Error clearing logs: {e}")
        return False

def start_scheduler():
    """Start the scheduler process directly, with environment variables from .env."""
    try:
        # Create logs directory if it doesn't exist
        logs_dir = os.path.dirname(SCHEDULER_LOG_FILE)
        os.makedirs(logs_dir, exist_ok=True)
        
        # Build the environment with API key from .env
        env = os.environ.copy()
        env_file = "/home/hunter/Desktop/tiny-ria/quotron/.env"
        if os.path.exists(env_file):
            log_message("Loading environment variables from .env file")
            with open(env_file, 'r') as f:
                for line in f:
                    if line.strip() and not line.startswith('#'):
                        key, value = line.strip().split('=', 1)
                        env[key] = value
                        
            # Log without revealing key
            if "ALPHA_VANTAGE_API_KEY" in env:
                log_message("Alpha Vantage API key loaded successfully")
            else:
                log_message("WARNING: Alpha Vantage API key not found in .env file")
        
        # Start the scheduler directly
        log_message("Starting scheduler")
        
        # Get the API scraper path
        api_scraper_path = "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/cmd/main"
        
        # Build the command
        cmd = [
            "go", "run", "cmd/scheduler/main.go",
            "-api-scraper=" + api_scraper_path
        ]
        
        # Start the process with redirected output
        with open(SCHEDULER_LOG_FILE, 'a') as log_file:
            process = subprocess.Popen(
                cmd,
                cwd="/home/hunter/Desktop/tiny-ria/quotron/scheduler",
                env=env,
                stdout=log_file,
                stderr=log_file,
                start_new_session=True
            )
        
        # Wait a moment to ensure process starts
        time.sleep(1)
        
        # Verify the process started
        if process.poll() is None:  # None means still running
            log_message("Scheduler started successfully with PID: " + str(process.pid))
            return True
        else:
            log_message(f"Scheduler failed to start, exit code: {process.poll()}")
            return False
    except Exception as e:
        log_message(f"Error starting scheduler: {e}")
        return False

def stop_scheduler():
    """Stop the scheduler process."""
    try:
        # Get the PIDs of any running scheduler processes
        result = subprocess.run(
            "ps aux | grep '[g]o run cmd/scheduler/main.go' | awk '{print $2}'", 
            shell=True, 
            capture_output=True, 
            text=True
        )
        
        pids = result.stdout.strip().split('\n')
        if pids and pids[0]:
            log_message(f"Found scheduler processes: {', '.join(pids)}")
            
            # Kill each process
            for pid in pids:
                if pid:
                    try:
                        subprocess.run(f"kill -9 {pid}", shell=True, check=True)
                        log_message(f"Killed process {pid}")
                    except subprocess.CalledProcessError as e:
                        log_message(f"Failed to kill process {pid}: {e}")
            
            log_message("Scheduler stopped")
            return True
        else:
            log_message("No scheduler processes found to stop")
            return False
    except Exception as e:
        log_message(f"Error stopping scheduler: {e}")
        return False

def run_job(job_name):
    """Run a specific job immediately."""
    try:
        log_message(f"Running job: {job_name}")
        
        # Build the environment with API key from .env
        env = os.environ.copy()
        env_file = "/home/hunter/Desktop/tiny-ria/quotron/.env"
        if os.path.exists(env_file):
            with open(env_file, 'r') as f:
                for line in f:
                    if line.strip() and not line.startswith('#'):
                        key, value = line.strip().split('=', 1)
                        env[key] = value
        
        # Get the API scraper path
        api_scraper_path = "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/cmd/main"
        
        # Build the command
        cmd = [
            "go", "run", "cmd/scheduler/main.go",
            "-api-scraper=" + api_scraper_path,
            "-run-job=" + job_name
        ]
        
        # Run the job and capture output
        result = subprocess.run(
            cmd,
            cwd="/home/hunter/Desktop/tiny-ria/quotron/scheduler",
            env=env,
            capture_output=True,
            text=True
        )
        
        # Log the output
        if result.stdout:
            log_message(f"Job {job_name} stdout: {result.stdout.strip()}")
        if result.stderr:
            log_message(f"Job {job_name} stderr: {result.stderr.strip()}")
            
        log_message(f"Job {job_name} completed with exit code {result.returncode}")
        return True
    except Exception as e:
        log_message(f"Error running job {job_name}: {e}")
        return False

# Data fetching
def get_latest_market_indices():
    """Get the latest market indices data."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT name as index_name, value, change, change_percent, timestamp
            FROM (
                SELECT *, ROW_NUMBER() OVER (PARTITION BY name ORDER BY timestamp DESC) as rn
                FROM market_indices
            ) sub
            WHERE rn = 1
            ORDER BY name
        """)
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_latest_stock_quotes(limit=20):
    """Get the latest stock quotes data."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT symbol, price, change, change_percent, volume, timestamp, source
            FROM (
                SELECT *, ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY timestamp DESC) as rn
                FROM stock_quotes
            ) sub
            WHERE rn = 1
            ORDER BY ABS(change_percent) DESC
            LIMIT %s
        """, (limit,))
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_investment_models():
    """Get all investment models."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT id, model_name as name, provider as description, fetched_at as created_at
            FROM investment_models
            ORDER BY model_name
        """)
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_model_holdings(model_id):
    """Get holdings for a specific investment model."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT ticker as symbol, position_name as name, allocation as weight, sector
            FROM model_holdings
            WHERE model_id = %s
            ORDER BY allocation DESC
        """, (model_id,))
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_sector_allocations(model_id):
    """Get sector allocations for a specific investment model."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT sector, allocation_percent as allocation
            FROM sector_allocations
            WHERE model_id = %s
            ORDER BY allocation_percent DESC
        """, (model_id,))
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_data_source_health():
    """Get health status of data sources."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        # Check the latest data from each source
        cur.execute("""
            SELECT source, 
                   MAX(timestamp) as last_update,
                   COUNT(*) as record_count,
                   NOW() - MAX(timestamp) as age
            FROM stock_quotes
            GROUP BY source
            ORDER BY last_update DESC
        """)
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

def get_batch_statistics():
    """Get statistics for data batches."""
    conn = get_db_connection()
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT b.id, b.source, b.created_at, b.status,
                   b.quote_count, b.index_count,
                   bs.positive_change_count, bs.negative_change_count, 
                   bs.mean_price, bs.mean_change_percent
            FROM data_batches b
            LEFT JOIN batch_statistics bs ON b.id = bs.batch_id
            ORDER BY b.created_at DESC
            LIMIT 10
        """)
        data = cur.fetchall()
        result = pd.DataFrame(data) if data else pd.DataFrame()
    
    conn.close()
    return result

# UI components
def render_scheduler_controls():
    """Render controls for the scheduler."""
    st.subheader("Scheduler Controls")
    
    # Check scheduler status with heartbeat
    scheduler_running = get_scheduler_status()
    status_color = "green" if scheduler_running else "red"
    status_text = "Running" if scheduler_running else "Stopped"
    
    # Status and controls section
    col1, col2, col3 = st.columns([2, 1, 1])
    with col1:
        st.markdown(f"**Status:** <span style='color:{status_color}'>{status_text}</span>", unsafe_allow_html=True)
        
        # Show last update time
        current_time = datetime.now().strftime("%H:%M:%S")
        st.markdown(f"**Last checked:** {current_time}")
    
    with col2:
        if scheduler_running:
            if st.button("Stop Scheduler", key="stop_scheduler"):
                if stop_scheduler():
                    st.success("Scheduler stopped successfully")
                    time.sleep(1)
                    st.rerun()
    
    with col3:
        if not scheduler_running:
            if st.button("Start Scheduler", key="start_scheduler"):
                if start_scheduler():
                    st.success("Scheduler started successfully")
                    time.sleep(1)
                    st.rerun()
        else:
            # Add a refresh button
            if st.button("Refresh Status", key="refresh_status"):
                st.rerun()
    
    # Job runner section
    st.divider()
    st.subheader("Run Individual Jobs")
    job_options = ["market_index_job", "stock_quote_job"]
    col1, col2 = st.columns([3, 1])
    with col1:
        selected_job = st.selectbox("Select a job to run", job_options)
    with col2:
        if st.button(f"Run", key="run_job"):
            if run_job(selected_job):
                st.success(f"Job {selected_job} executed successfully")
                time.sleep(1)
                st.rerun()  # Refresh to show new logs
                
    # Scheduler logs section with manual refresh
    st.divider()
    
    # Header with refresh and clear buttons
    log_col1, log_col2, log_col3 = st.columns([3, 1, 1])
    with log_col1:
        st.subheader("Scheduler Logs")
    with log_col2:
        if st.button("Refresh Logs", key="refresh_logs"):
            st.rerun()
    with log_col3:
        if st.button("Clear Logs", key="clear_logs"):
            if clear_logs():
                st.success("Logs cleared successfully")
                time.sleep(1)
                st.rerun()
    
    # Display logs in a scrollable area
    logs = get_scheduler_logs(30)  # Get last 30 log entries
    log_text = "".join(logs)
    st.code(log_text, language="bash")

def render_market_overview():
    """Render the market overview section."""
    st.subheader("Market Overview")
    
    indices = get_latest_market_indices()
    if not indices.empty:
        for _, index in indices.iterrows():
            col1, col2, col3 = st.columns([2, 2, 3])
            with col1:
                st.markdown(f"**{index['index_name']}**")
            with col2:
                st.markdown(f"{index['value']:.2f}")
            with col3:
                change = float(index['change'])
                change_pct = float(index['change_percent'])
                color = "green" if change >= 0 else "red"
                st.markdown(f"<span style='color:{color}'>{'▲' if change >= 0 else '▼'} {change:.2f} ({change_pct:.2f}%)</span>", 
                            unsafe_allow_html=True)
    else:
        st.info("No market index data available.")
    
    st.divider()
    
    st.subheader("Top Movers")
    
    stocks = get_latest_stock_quotes()
    if not stocks.empty:
        display_cols = ['symbol', 'price', 'change', 'change_percent', 'source']
        stocks_formatted = stocks[display_cols].copy()
        
        # Apply formatting
        stocks_formatted['price'] = stocks_formatted['price'].apply(lambda x: f"${float(x):.2f}")
        
        # Apply color coding for change and change_percent
        def color_change(val):
            try:
                val = float(val)
                color = 'green' if val >= 0 else 'red'
                return f'color: {color}'
            except:
                return ''
        
        st.dataframe(
            stocks_formatted.style.map(color_change, subset=['change', 'change_percent']),
            use_container_width=True
        )
    else:
        st.info("No stock quote data available.")

def render_data_source_health():
    """Render the data source health section."""
    st.subheader("Data Source Health")
    
    health_data = get_data_source_health()
    if not health_data.empty:
        for _, source in health_data.iterrows():
            col1, col2, col3 = st.columns([2, 2, 3])
            with col1:
                st.markdown(f"**{source['source']}**")
            with col2:
                age_minutes = source['age'].total_seconds() / 60
                status = "Healthy" if age_minutes < 60 else "Stale"
                color = "green" if status == "Healthy" else "red"
                st.markdown(f"Status: <span style='color:{color}'>{status}</span>", unsafe_allow_html=True)
            with col3:
                st.markdown(f"Last Update: {source['last_update'].strftime('%Y-%m-%d %H:%M:%S')}")
                st.markdown(f"Records: {source['record_count']}")
    else:
        st.info("No data source health information available.")
    
    st.divider()
    
    st.subheader("Recent Batch Statistics")
    
    batch_stats = get_batch_statistics()
    if not batch_stats.empty:
        st.dataframe(batch_stats, use_container_width=True)
    else:
        st.info("No batch statistics available.")

def render_investment_models():
    """Render the investment models section."""
    st.subheader("Investment Models")
    
    models = get_investment_models()
    if not models.empty:
        selected_model_name = st.selectbox(
            "Select a model",
            models['name'].tolist()
        )
        
        selected_model = models[models['name'] == selected_model_name].iloc[0]
        st.markdown(f"**Description:** {selected_model['description']}")
        
        # Get holdings and sector allocations
        holdings = get_model_holdings(selected_model['id'])
        sectors = get_sector_allocations(selected_model['id'])
        
        col1, col2 = st.columns(2)
        with col1:
            st.markdown("#### Sector Allocation")
            if not sectors.empty:
                fig = px.pie(sectors, values='allocation', names='sector', 
                            title=f"{selected_model_name} Sector Allocation")
                fig.update_traces(textposition='inside', textinfo='percent+label')
                st.plotly_chart(fig, use_container_width=True)
            else:
                st.info("No sector allocation data available.")
        
        with col2:
            st.markdown("#### Top Holdings")
            if not holdings.empty:
                top_holdings = holdings.head(10)
                fig = px.bar(top_holdings, x='weight', y='symbol', 
                            title=f"{selected_model_name} Top Holdings",
                            orientation='h')
                fig.update_layout(yaxis={'categoryorder':'total ascending'})
                st.plotly_chart(fig, use_container_width=True)
            else:
                st.info("No holdings data available.")
        
        # Display full holdings table
        if not holdings.empty:
            st.markdown("#### All Holdings")
            st.dataframe(holdings, use_container_width=True)
    else:
        st.info("No investment models available.")

# Main app
def show_db_connection_settings():
    """Show database connection settings and allow editing."""
    st.subheader("Database Connection Settings")
    
    # Add a "Test Connection" button right at the top
    if st.button("Test Current Connection", key="test_current_connection"):
        # This will either work or show a clear error in Streamlit
        conn = psycopg2.connect(
            host=os.environ["DB_HOST"],
            port=os.environ["DB_PORT"],
            database=os.environ["DB_NAME"],
            user=os.environ["DB_USER"],
            password=os.environ["DB_PASSWORD"]
        )
        conn.close()
        st.success("✅ Connection successful!")
    
    # Display current settings
    current_host = os.environ["DB_HOST"]
    current_port = os.environ["DB_PORT"]
    current_name = os.environ["DB_NAME"]
    current_user = os.environ["DB_USER"]
    current_password = os.environ["DB_PASSWORD"]
    
    # Allow editing
    col1, col2 = st.columns(2)
    with col1:
        new_host = st.text_input("Database Host", current_host)
        new_port = st.text_input("Database Port", current_port)
        new_name = st.text_input("Database Name", current_name)
    
    with col2:
        new_user = st.text_input("Database User", current_user)
        new_password = st.text_input("Database Password", current_password, type="password")
    
    # Test connection button
    test_conn = st.button("Test New Settings", key="test_new_settings")
    
    if test_conn:
        try:
            # Test connection with new settings
            conn = psycopg2.connect(
                host=new_host,
                port=new_port,
                database=new_name,
                user=new_user,
                password=new_password
            )
            conn.close()
            st.success("Connection successful!")
            
            # Update settings for current session
            os.environ["DB_HOST"] = new_host
            os.environ["DB_PORT"] = new_port
            os.environ["DB_NAME"] = new_name
            os.environ["DB_USER"] = new_user
            os.environ["DB_PASSWORD"] = new_password
            
            # Save settings button appears after successful test
            save_settings = st.button("Save Settings to .env", key="save_settings")
            
            if save_settings:
                env_file = ".env"
                with open(env_file, "w") as f:
                    f.write(f"DB_HOST={new_host}\n")
                    f.write(f"DB_PORT={new_port}\n")
                    f.write(f"DB_NAME={new_name}\n")
                    f.write(f"DB_USER={new_user}\n")
                    f.write(f"DB_PASSWORD={new_password}\n")
                    f.write(f"SCHEDULER_PATH={os.environ.get('SCHEDULER_PATH', '/home/hunter/Desktop/tiny-ria/quotron/scheduler')}\n")
                st.success("Settings saved to .env file!")
                st.info("Please restart the application for settings to take full effect.")
                
        except Exception as e:
            st.error(f"Connection failed: {e}")
            
    # Information box with PostgreSQL commands 
    with st.expander("PostgreSQL Connection Help"):
        st.markdown("""
        ### Common PostgreSQL issues:
        
        1. **Check PostgreSQL service status:**
           ```bash
           sudo systemctl status postgresql
           ```
        
        2. **Restart PostgreSQL service:**
           ```bash
           sudo systemctl restart postgresql
           ```
        
        3. **Test connection with psql:**
           ```bash
           PGPASSWORD=quotron psql -U quotron -h localhost -d quotron -c "SELECT 1"
           ```
        
        4. **Check for tables:**
           ```bash
           PGPASSWORD=quotron psql -U quotron -h localhost -d quotron -c "\\dt"
           ```
           
        5. **Verify model data exists:**
           ```bash
           PGPASSWORD=quotron psql -U quotron -h localhost -d quotron -c "SELECT COUNT(*) FROM model_holdings"
           ```
        """)
        
    # Check database tables
    st.divider()
    st.subheader("Database Table Check")
    if st.button("Check Database Tables", key="check_tables"):
        conn = get_db_connection()
        with conn.cursor() as cur:
            cur.execute("""
                SELECT table_name 
                FROM information_schema.tables 
                WHERE table_schema = 'public'
                ORDER BY table_name
            """)
            tables = [row[0] for row in cur.fetchall()]
            if tables:
                st.success(f"Found {len(tables)} tables in database")
                st.write("Available tables:")
                for table in tables:
                    st.code(table)
            else:
                st.warning("No tables found in the database.")
        conn.close()

def main():
    st.set_page_config(
        page_title="Quotron Dashboard",
        page_icon="📊",
        layout="wide",
    )

    st.title("Quotron Financial Data Dashboard")
    
    # Tabs for different sections
    tab1, tab2, tab3, tab4 = st.tabs(["Market Data", "Data Sources", "Investment Models", "Settings"])
    
    with tab1:
        col1, col2 = st.columns([1, 2])
        
        with col1:
            render_scheduler_controls()
        
        with col2:
            render_market_overview()
    
    with tab2:
        render_data_source_health()
    
    with tab3:
        render_investment_models()
        
    with tab4:
        show_db_connection_settings()

if __name__ == "__main__":
    main()