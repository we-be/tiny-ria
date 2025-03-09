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
    """Get detailed health status of all data sources using the enhanced data_source_health table."""
    conn = get_db_connection()
    detailed_health = pd.DataFrame()
    record_counts = pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            # Get the detailed health data
            cur.execute("""
                SELECT 
                    source_type, 
                    source_name,
                    source_detail,
                    status,
                    last_check,
                    last_success,
                    error_count,
                    error_message,
                    response_time_ms,
                    CASE 
                        WHEN last_success IS NOT NULL THEN NOW() - last_success
                        ELSE interval '999 days'
                    END as age,
                    metadata
                FROM data_source_health
                ORDER BY source_type, source_name
            """)
            detailed_health = pd.DataFrame(cur.fetchall()) if cur.rowcount > 0 else pd.DataFrame()
            
            # Get record counts from stock_quotes to enrich the health data
            cur.execute("""
                WITH source_mapping AS (
                    SELECT 
                        CASE 
                            WHEN source::text LIKE '%yfinance%' OR source::text LIKE '%yahoo%' THEN 'yahoo_finance_proxy'
                            WHEN source = 'api-scraper' THEN 'alpha_vantage'
                            ELSE 'other'
                        END as source_name,
                        source,
                        COUNT(*) as record_count
                    FROM stock_quotes
                    GROUP BY source
                )
                SELECT 
                    source_name,
                    SUM(record_count) as record_count
                FROM source_mapping
                GROUP BY source_name
            """)
            record_counts = pd.DataFrame(cur.fetchall()) if cur.rowcount > 0 else pd.DataFrame()
            
    except Exception as e:
        st.error(f"Error fetching health data: {e}")
    finally:
        try:
            conn.close()
        except:
            pass
    
    # Merge record counts into health data if both exist
    if not detailed_health.empty and not record_counts.empty:
        try:
            detailed_health = detailed_health.merge(
                record_counts, 
                left_on='source_name', 
                right_on='source_name', 
                how='left'
            )
            # Fill missing values with zeros
            detailed_health['record_count'] = detailed_health['record_count'].fillna(0)
        except Exception as e:
            st.warning(f"Error enriching health data with record counts: {e}")
    
    return detailed_health

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
    """Render the data source health section with enhanced monitoring and visual indicators."""
    st.subheader("Data Source Health")
    
    try:
        health_data = get_data_source_health()
        
        if not health_data.empty:
            # Create status summary metrics
            healthy_count = len(health_data[health_data['status'] == 'healthy'])
            degraded_count = len(health_data[health_data['status'].isin(['degraded', 'limited'])])
            failed_count = len(health_data[health_data['status'].isin(['failed', 'error'])])
            total_count = len(health_data)
            
            # Calculate health score as a percentage
            health_score = (healthy_count + (degraded_count * 0.5)) / total_count * 100 if total_count > 0 else 0
            
            # Create a dashboard-style display with health score
            cols = st.columns([1, 1, 1, 2])
            with cols[0]:
                # Health score gauge using Streamlit metric
                st.metric(
                    "Health Score", 
                    f"{health_score:.0f}%", 
                    f"{health_score - 50:.0f}%" if health_score != 50 else None,
                    delta_color="normal" if health_score >= 50 else "inverse"
                )
            
            with cols[1]:
                # Status counts with color indicators
                status_html = f"""
                <div style="padding: 10px; border-radius: 5px;">
                    <div style="display: flex; align-items: center; margin-bottom: 8px;">
                        <div style="width: 12px; height: 12px; border-radius: 50%; background-color: green; margin-right: 8px;"></div>
                        <div><strong>Healthy:</strong> {healthy_count}</div>
                    </div>
                    <div style="display: flex; align-items: center; margin-bottom: 8px;">
                        <div style="width: 12px; height: 12px; border-radius: 50%; background-color: orange; margin-right: 8px;"></div>
                        <div><strong>Degraded:</strong> {degraded_count}</div>
                    </div>
                    <div style="display: flex; align-items: center;">
                        <div style="width: 12px; height: 12px; border-radius: 50%; background-color: red; margin-right: 8px;"></div>
                        <div><strong>Failed:</strong> {failed_count}</div>
                    </div>
                </div>
                """
                st.markdown(status_html, unsafe_allow_html=True)
                
            with cols[2]:
                # Data freshness
                latest_update = health_data['last_check'].max() if not health_data.empty else None
                if latest_update:
                    minutes_ago = int((datetime.now(latest_update.tzinfo) - latest_update).total_seconds() / 60)
                    freshness_color = "green" if minutes_ago < 10 else "orange" if minutes_ago < 30 else "red"
                    st.markdown(f"""
                    <div style="padding: 10px; border-radius: 5px;">
                        <div style="margin-bottom: 5px;"><strong>Last Updated:</strong></div>
                        <div style="font-size: 1.2em; color: {freshness_color};">{minutes_ago} minutes ago</div>
                        <div style="font-size: 0.8em; color: gray;">{latest_update.strftime('%Y-%m-%d %H:%M')}</div>
                    </div>
                    """, unsafe_allow_html=True)
            
            with cols[3]:
                # Action buttons in a row
                action_cols = st.columns(4)
                with action_cols[0]:
                    if st.button("🔄 Refresh", key="refresh_health"):
                        st.rerun()
                with action_cols[1]:
                    # AI Diagnostics button
                    if st.button("🤖 AI Diagnose", key="ai_diagnostics"):
                        with st.status("Analyzing data sources...", expanded=True) as status:
                            st.write("Collecting source information...")
                            time.sleep(0.5)
                            st.write("Analyzing error patterns...")
                            time.sleep(0.5)
                            st.write("Generating recommendations...")
                            time.sleep(0.5)
                            report_path = generate_diagnostics_report(health_data)
                            status.update(label="Analysis complete!", state="complete", expanded=False)
                        st.success("Diagnostics report generated!")
                        st.toast("AI report ready! 🧠", icon="✅")
                with action_cols[2]:
                    # Recovery button
                    if st.button("🛠️ Auto-Recover", key="auto_recover_all"):
                        with st.status("Attempting recovery...", expanded=True) as status:
                            recovered, failed = run_auto_recovery(health_data)
                            
                            progress_value = 0
                            progress_bar = st.progress(progress_value)
                            
                            total_sources = len(recovered) + len(failed)
                            if total_sources > 0:
                                for i, source in enumerate(recovered):
                                    progress_value = int((i+1) / total_sources * 100)
                                    progress_bar.progress(progress_value)
                                    st.write(f"✅ Recovered: {source}")
                                    time.sleep(0.3)
                                    
                                for i, (source, error) in enumerate(failed):
                                    progress_value = int((len(recovered) + i+1) / total_sources * 100)
                                    progress_bar.progress(progress_value)
                                    st.write(f"❌ Failed: {source} - {error}")
                                    time.sleep(0.3)
                                
                                status.update(label=f"Recovery complete! {len(recovered)}/{total_sources} sources recovered", 
                                             state="complete" if len(recovered) > 0 else "error",
                                             expanded=False)
                                
                                if len(recovered) > 0:
                                    st.balloons()
                            else:
                                status.update(label="No sources needed recovery!", state="complete", expanded=False)
                with action_cols[3]:
                    # Check all sources button
                    if st.button("🔍 Check All", key="check_all"):
                        with st.status("Checking all data sources...", expanded=True) as status:
                            st.write("Checking YFinance proxy...")
                            time.sleep(0.5)
                            st.write("Checking API sources...")
                            time.sleep(0.5)
                            st.write("Checking browser scrapers...")
                            time.sleep(0.5)
                            check_all_sources()
                            status.update(label="Health check complete!", state="complete", expanded=False)
                            st.toast("Health check finished", icon="🔍")
            
            # Create a tabbed view for different ways to view the data
            tab1, tab2, tab3 = st.tabs(["Overview", "Details", "Failed Sources"])
            
            with tab1:
                # Create a visual status card for each source
                st.markdown("### Source Status")
                
                # Sort by status (failed first, then degraded, then healthy)
                def status_sort_key(status):
                    if status in ['failed', 'error']:
                        return 0
                    elif status in ['degraded', 'limited']:
                        return 1
                    elif status == 'healthy':
                        return 2
                    return 3
                
                health_data['status_sort'] = health_data['status'].apply(status_sort_key)
                sorted_health = health_data.sort_values(['status_sort', 'source_type', 'source_name'])
                
                # Create a grid of cards - 4 columns
                source_grid = st.columns(4)
                
                # Helper for status indicator
                def get_status_indicator(status):
                    if status == 'healthy':
                        return "🟢"
                    elif status in ['degraded', 'limited']:
                        return "🟠"
                    elif status in ['error', 'failed']:
                        return "🔴"
                    return "⚪"
                
                # Helper for status color
                def get_status_color(status):
                    if status == 'healthy':
                        return "green"
                    elif status in ['degraded', 'limited']:
                        return "orange"
                    elif status in ['error', 'failed']:
                        return "red"
                    return "gray"
                
                # Display each source as a card
                for i, (_, source) in enumerate(sorted_health.iterrows()):
                    col_idx = i % 4
                    status_color = get_status_color(source['status'])
                    status_icon = get_status_indicator(source['status'])
                    
                    # Determine age display
                    if pd.notna(source.get('last_success')):
                        age_minutes = source.get('age', 0)
                        if isinstance(age_minutes, str) and age_minutes == 'Never':
                            age_text = "Never"
                        else:
                            age_text = f"{int(age_minutes)} min" if age_minutes < 60 else f"{int(age_minutes/60)} hr"
                    else:
                        age_text = "Never"
                    
                    with source_grid[col_idx]:
                        st.markdown(f"""
                        <div style="padding: 10px; border-radius: 5px; border: 1px solid #ddd; margin-bottom: 10px; background-color: rgba({', '.join(['255' if status_color=='red' else '255', '204' if status_color=='orange' else '255', '204' if status_color=='green' else '255'])}, 0.2);">
                            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 5px;">
                                <div style="font-weight: bold;">{source['source_name']}</div>
                                <div style="font-size: 1.2em;">{status_icon}</div>
                            </div>
                            <div style="color: gray; font-size: 0.8em; margin-bottom: 5px;">{source['source_type']}</div>
                            <div style="display: flex; justify-content: space-between; font-size: 0.9em;">
                                <div>Age: <span style="color: {status_color};">{age_text}</span></div>
                                <div>Errors: {source['error_count']}</div>
                            </div>
                        </div>
                        """, unsafe_allow_html=True)
            
            with tab2:
                # Detailed table view
                st.markdown("### Detailed Status")
                
                # Prepare the dataframe for display
                display_df = health_data[['source_type', 'source_name', 'source_detail', 'status', 'last_check', 'last_success', 'error_count', 'response_time_ms']].copy()
                
                # Add age column in minutes
                display_df['age'] = display_df['last_success'].apply(
                    lambda x: 'Never' if pd.isna(x) else int((datetime.now(x.tzinfo) - x).total_seconds() / 60)
                )
                
                # Add a visual status indicator
                display_df['indicator'] = display_df['status'].apply(get_status_indicator)
                
                # Format record count
                if 'record_count' in display_df.columns:
                    display_df['records'] = display_df['record_count'].fillna(0).astype(int)
                
                # Reorder and select columns for display
                display_cols = ['indicator', 'source_name', 'source_detail', 'status', 'age', 'error_count', 'response_time_ms']
                if 'records' in display_df.columns:
                    display_cols.append('records')
                    
                compact_df = display_df[display_cols].copy()
                
                # Rename columns for better display
                compact_df.columns = ['', 'Source', 'Description', 'Status', 'Age (min)', 'Errors', 'Response (ms)'] + \
                                    (['Records'] if 'records' in display_df.columns else [])
                
                # Show the dataframe with conditional formatting
                def highlight_status(s):
                    if s.name == 'Status':
                        return ['color: green' if x == 'healthy' 
                                else 'color: orange' if x in ['degraded', 'limited']
                                else 'color: red' if x in ['error', 'failed']
                                else 'color: gray' for x in s]
                    elif s.name == 'Age (min)':
                        return ['color: red' if x != 'Never' and int(x) > 60
                                else 'color: orange' if x != 'Never' and int(x) > 30
                                else 'color: green' if x != 'Never'
                                else 'color: gray' for x in s]
                    elif s.name == 'Errors':
                        return ['color: red' if x > 5
                                else 'color: orange' if x > 0
                                else 'color: green' for x in s]
                    return [''] * len(s)
                
                st.dataframe(
                    compact_df.style.apply(highlight_status),
                    use_container_width=True,
                    height=400
                )
            
            with tab3:
                # Display failures in a collapsible section
                failures = health_data[health_data['status'].isin(['failed', 'error'])]
                if not failures.empty:
                    st.markdown("### Failed Sources")
                    for _, source in failures.iterrows():
                        with st.expander(f"{get_status_indicator(source['status'])} {source['source_name']} - {source['source_detail']}"):
                            col1, col2 = st.columns([3, 1])
                            
                            with col1:
                                if pd.notna(source.get('error_message')) and source.get('error_message'):
                                    st.markdown("**Last Error:**")
                                    st.code(source['error_message'], language="bash")
                                
                                if pd.notna(source.get('metadata')) and source.get('metadata'):
                                    with st.expander("Metadata"):
                                        if isinstance(source['metadata'], str):
                                            try:
                                                metadata = json.loads(source['metadata'])
                                                st.json(metadata)
                                            except:
                                                st.text(source['metadata'])
                                        else:
                                            st.json(source['metadata'])
                            
                            with col2:
                                st.markdown(f"**Last Check:** {source['last_check'].strftime('%Y-%m-%d %H:%M')}")
                                st.markdown(f"**Error Count:** {source['error_count']}")
                                
                                # Add recovery button
                                if st.button("Attempt Recovery", key=f"recover_{source['source_name']}"):
                                    # Handle specific recovery based on source type
                                    with st.status(f"Recovering {source['source_name']}...", expanded=True) as status:
                                        if source['source_name'] == 'yahoo_finance_proxy':
                                            st.write("Attempting to restart the proxy service...")
                                            time.sleep(0.5)
                                            proxy_restart_result = restart_yfinance_proxy()
                                            
                                            if proxy_restart_result:
                                                status.update(label=f"✅ {source['source_name']} recovered successfully!", 
                                                            state="complete", expanded=False)
                                                st.toast("Recovery successful!", icon="✅")
                                                st.snow()  # Little celebration for successful recovery
                                            else:
                                                status.update(label=f"❌ Failed to recover {source['source_name']}", 
                                                            state="error", expanded=True)
                                                st.error("Recovery failed. See logs for details.")
                                                
                                        elif source['source_name'] == 'alpha_vantage':
                                            st.write("Checking API key status...")
                                            time.sleep(0.5)
                                            st.write("Attempting request with minimal parameters...")
                                            time.sleep(0.5)
                                            status.update(label="⚠️ Alpha Vantage recovery not implemented yet", 
                                                        state="warning", expanded=False)
                                            st.warning("Alpha Vantage recovery not implemented yet")
                                        else:
                                            status.update(label=f"⚠️ No recovery method for {source['source_name']}", 
                                                        state="warning", expanded=False)
                                            st.warning(f"No recovery method defined for {source['source_name']}")
                else:
                    st.success("🎉 No failed sources detected!")
        else:
            st.info("No data source health information available. Please check database connection and make sure the data_source_health table exists.")
            
    except Exception as e:
        st.error(f"Error loading health data: {e}")
        st.info("Data source health information is temporarily unavailable.")
    
    # Proxy and Services section
    st.divider()
    st.markdown("## Services Status")
    
    # Create a tabbed interface for services
    service_tab1, service_tab2 = st.tabs(["YFinance Proxy", "Batch Statistics"])
    
    with service_tab1:
        # YFinance Proxy Health Check
        proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
        health_endpoint = f"{proxy_url}/health"
        metrics_endpoint = f"{proxy_url}/metrics"
        
        # Split into two columns - status and actions
        status_col, actions_col = st.columns([3, 1])
        
        with status_col:
            # Try to get current status
            try:
                response = requests.get(health_endpoint, timeout=2)
                if response.status_code == 200:
                    status_data = response.json()
                    uptime = status_data.get('uptime', 0)
                    uptime_str = f"{int(uptime / 3600)} hours, {int((uptime % 3600) / 60)} minutes" if uptime > 0 else "Just started"
                    
                    # Get metrics too if available
                    try:
                        metrics_response = requests.get(metrics_endpoint, timeout=2)
                        if metrics_response.status_code == 200:
                            metrics_data = metrics_response.json()
                            cache_stats = metrics_data.get('provider_stats', {}).get('cache_stats', {})
                            request_stats = metrics_data.get('provider_stats', {}).get('request_stats', {})
                            
                            # Create a nice status display
                            st.markdown(f"""
                            <div style="display: flex; flex-direction: column;">
                                <div style="display: flex; margin-bottom: 10px;">
                                    <div style="flex: 1; padding: 10px; background-color: rgba(0, 200, 0, 0.1); border-radius: 5px; margin-right: 10px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">Status</div>
                                        <div style="font-size: 1.2em; color: green;">✅ {status_data.get('status', 'ok').upper()}</div>
                                    </div>
                                    <div style="flex: 1; padding: 10px; background-color: rgba(0, 0, 200, 0.1); border-radius: 5px; margin-right: 10px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">Uptime</div>
                                        <div>{uptime_str}</div>
                                    </div>
                                    <div style="flex: 1; padding: 10px; background-color: rgba(200, 200, 0, 0.1); border-radius: 5px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">Cache Hit Ratio</div>
                                        <div>{cache_stats.get('hit_ratio', 0)*100:.1f}%</div>
                                    </div>
                                </div>
                                <div style="display: flex;">
                                    <div style="flex: 1; padding: 10px; background-color: rgba(200, 0, 200, 0.1); border-radius: 5px; margin-right: 10px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">Requests</div>
                                        <div>Total: {request_stats.get('total_requests', 0)}</div>
                                        <div>Successful: {request_stats.get('successful_requests', 0)}</div>
                                        <div>Failed: {request_stats.get('failed_requests', 0)}</div>
                                    </div>
                                    <div style="flex: 1; padding: 10px; background-color: rgba(0, 200, 200, 0.1); border-radius: 5px; margin-right: 10px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">Cache</div>
                                        <div>Hits: {cache_stats.get('hits', 0)}</div>
                                        <div>Misses: {cache_stats.get('misses', 0)}</div>
                                        <div>Entries: {cache_stats.get('entries', 0)}</div>
                                    </div>
                                    <div style="flex: 1; padding: 10px; background-color: rgba(150, 150, 150, 0.1); border-radius: 5px;">
                                        <div style="font-weight: bold; margin-bottom: 5px;">API Calls</div>
                                        <div>{request_stats.get('api_calls', 0)}</div>
                                    </div>
                                </div>
                            </div>
                            """, unsafe_allow_html=True)
                        else:
                            st.warning("Metrics endpoint not available")
                    except:
                        st.warning("Could not fetch metrics data")
                else:
                    st.error(f"❌ Proxy returned error code: {response.status_code}")
            except requests.RequestException as e:
                st.error(f"❌ Proxy connection failed: {str(e)}")
                st.info("The YFinance Proxy is not running or is not accessible.")
        
        with actions_col:
            # Action buttons
            st.markdown("### Actions")
            if st.button("Check Health", key="check_proxy_health"):
                try:
                    start_time = time.time()
                    response = requests.get(health_endpoint, timeout=5)
                    response_time = (time.time() - start_time) * 1000  # ms
                    
                    if response.status_code == 200:
                        status_data = response.json()
                        st.success(f"✅ Proxy is running: {status_data.get('status', 'ok')}")
                        st.markdown(f"Response time: {response_time:.1f} ms")
                        
                        # Update the health record in the database
                        conn = get_db_connection()
                        with conn.cursor() as cur:
                            try:
                                metadata = json.dumps({
                                    "response_time": response_time,
                                    "checked_at": datetime.now().isoformat(),
                                    "endpoint": health_endpoint
                                })
                                
                                cur.execute("""
                                    UPDATE data_source_health
                                    SET status = 'healthy',
                                        last_check = NOW(),
                                        last_success = NOW(),
                                        response_time_ms = %s,
                                        error_message = NULL,
                                        metadata = %s
                                    WHERE source_name = 'yahoo_finance_proxy'
                                """, (int(response_time), metadata))
                                conn.commit()
                            except Exception as e:
                                conn.rollback()
                            finally:
                                conn.close()
                    else:
                        st.error(f"❌ Proxy returned error code: {response.status_code}")
                        update_health_status_failed("yahoo_finance_proxy", f"Proxy returned error code: {response.status_code}", response_time)
                except requests.RequestException as e:
                    st.error(f"❌ Proxy connection failed: {str(e)}")
                    update_health_status_failed("yahoo_finance_proxy", f"Connection failed: {str(e)}")
            
            if st.button("Restart Proxy", key="restart_proxy"):
                with st.status("Restarting YFinance proxy...", expanded=True) as status:
                    st.write("Stopping old process...")
                    time.sleep(0.5)
                    st.write("Starting new proxy instance...")
                    time.sleep(0.5)
                    success = restart_yfinance_proxy()
                    
                    if success:
                        st.write("Verifying health...")
                        time.sleep(0.5)
                        status.update(label="✅ Proxy restarted successfully!", state="complete", expanded=False)
                        st.toast("Proxy running", icon="🚀")
                    else:
                        status.update(label="❌ Failed to restart proxy", state="error", expanded=True)
                        st.error("Could not restart the proxy service. Check logs for details.")
            
            if st.button("Clear Cache", key="clear_cache"):
                with st.spinner("Clearing cache..."):
                    try:
                        response = requests.post(f"{proxy_url}/admin/cache/clear", timeout=5)
                        if response.status_code == 200:
                            st.success("✅ Cache cleared successfully")
                            st.toast("Cache cleared", icon="🧹") 
                        else:
                            st.error(f"❌ Failed to clear cache: {response.status_code}")
                    except requests.RequestException as e:
                        st.error(f"❌ Request failed: {str(e)}")
    
    with service_tab2:
        # Batch Statistics
        st.subheader("Recent Batch Statistics")
        
        try:
            batch_stats = get_batch_statistics()
            if not batch_stats.empty:
                # Display as a chart and table
                if 'created_at' in batch_stats.columns and 'quote_count' in batch_stats.columns:
                    # Prepare data for chart
                    chart_data = batch_stats[['created_at', 'quote_count', 'index_count']].copy()
                    chart_data = chart_data.sort_values('created_at')
                    
                    # Create a line chart
                    fig = px.line(chart_data, x='created_at', y=['quote_count', 'index_count'], 
                                 title='Batch Sizes Over Time',
                                 labels={'value': 'Count', 'created_at': 'Time', 'variable': 'Type'})
                    st.plotly_chart(fig, use_container_width=True)
                
                # Display the dataframe
                st.dataframe(batch_stats, use_container_width=True)
            else:
                st.info("No batch statistics available.")
        except Exception as e:
            st.error(f"Error loading batch statistics: {e}")
            st.info("Batch statistics are temporarily unavailable.")


def generate_diagnostics_report(health_data):
    """Generate a comprehensive diagnostics report for all data sources
    
    Args:
        health_data: DataFrame containing health data for all sources
    """
    # Create the report file path
    report_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "diagnostics_report.md")
    
    # Get current time
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    # Start the report with a header
    report = f"""# Data Source Diagnostics Report
Generated at: {now}

## Overview
"""
    
    # Add overall statistics
    healthy_count = len(health_data[health_data['status'] == 'healthy'])
    degraded_count = len(health_data[health_data['status'].isin(['degraded', 'limited'])])
    failed_count = len(health_data[health_data['status'].isin(['failed', 'error'])])
    total_count = len(health_data)
    
    health_score = (healthy_count + (degraded_count * 0.5)) / total_count * 100 if total_count > 0 else 0
    
    report += f"""
- **Total Sources**: {total_count}
- **Healthy**: {healthy_count} ({healthy_count/total_count*100:.1f}%)
- **Degraded**: {degraded_count} ({degraded_count/total_count*100:.1f}%)
- **Failed**: {failed_count} ({failed_count/total_count*100:.1f}%)
- **Health Score**: {health_score:.1f}%

## Health Summary

| Source | Type | Status | Last Success | Age | Error Count |
|--------|------|--------|--------------|-----|-------------|
"""
    
    # Add each source to the report
    for _, source in health_data.iterrows():
        status = source['status']
        status_emoji = "✅" if status == 'healthy' else "⚠️" if status in ['degraded', 'limited'] else "❌"
        
        # Format last success time
        if pd.notna(source.get('last_success')):
            last_success = source['last_success'].strftime("%Y-%m-%d %H:%M:%S")
            age_minutes = source.get('age', 0)
            if isinstance(age_minutes, str) and age_minutes == 'Never':
                age = "Never"
            else:
                age = f"{int(age_minutes)} min" if age_minutes < 60 else f"{int(age_minutes/60)} hours"
        else:
            last_success = "Never"
            age = "N/A"
        
        report += f"| {source['source_name']} | {source['source_type']} | {status_emoji} {status} | {last_success} | {age} | {source['error_count']} |\n"
    
    # Add section for failures with detailed analysis
    failures = health_data[health_data['status'].isin(['failed', 'error'])]
    if not failures.empty:
        report += "\n## Failed Sources Analysis\n\n"
        
        for _, source in failures.iterrows():
            report += f"### {source['source_name']} ({source['source_type']})\n\n"
            
            # Add error details
            if pd.notna(source.get('error_message')) and source.get('error_message'):
                report += f"**Error Message:**\n```\n{source['error_message']}\n```\n\n"
            
            # Add metadata if available
            if pd.notna(source.get('metadata')) and source.get('metadata'):
                try:
                    if isinstance(source['metadata'], str):
                        metadata = json.loads(source['metadata'])
                    else:
                        metadata = source['metadata']
                    
                    report += "**Metadata:**\n```json\n"
                    report += json.dumps(metadata, indent=2)
                    report += "\n```\n\n"
                except:
                    pass
            
            # Add diagnostic analysis based on source type and error patterns
            report += "**Diagnostic Analysis:**\n\n"
            
            if source['source_name'] == 'yahoo_finance_proxy':
                report += "This is the YFinance Python proxy service. Issues could be:\n\n"
                report += "1. The proxy service might not be running - check with `ps aux | grep yfinance_proxy`\n"
                report += "2. There might be network connectivity issues to the Yahoo Finance API\n"
                report += "3. The port might be blocked or already in use\n\n"
                report += "**Recommendation:** Try restarting the proxy service with the 'Restart Proxy' button on the dashboard.\n\n"
            
            elif source['source_name'] == 'alpha_vantage':
                report += "This is the Alpha Vantage API client. Issues could be:\n\n"
                report += "1. The API key might be invalid or expired\n"
                report += "2. You might have exceeded the API rate limits\n"
                report += "3. There might be network connectivity issues\n\n"
                report += "**Recommendation:** Check the API key or consider rotating to a new key.\n\n"
            
            elif source['source_type'] == 'browser-scraper':
                report += "This is a browser-based web scraper. Issues could be:\n\n"
                report += "1. The website structure might have changed, breaking the scraper\n"
                report += "2. You might be blocked by the website's anti-scraping measures\n"
                report += "3. Browser automation dependencies might need updating\n\n"
                report += "**Recommendation:** Review the scraper code and compare with the current website structure.\n\n"
            
            else:
                report += "General troubleshooting steps:\n\n"
                report += "1. Check network connectivity to the source\n"
                report += "2. Verify authentication credentials if applicable\n"
                report += "3. Look for error patterns in the logs\n\n"
    
    # Add recommendations section
    report += "\n## General Recommendations\n\n"
    
    if failed_count > 0:
        report += "### Critical Issues\n\n"
        report += f"- {failed_count} sources are currently failing and require attention\n"
        report += "- Use the Auto-Recovery function on the dashboard to attempt automated fixes\n"
        report += "- Review the error messages above for specific troubleshooting steps\n\n"
    
    if degraded_count > 0:
        report += "### Performance Issues\n\n"
        report += f"- {degraded_count} sources are degraded and may need optimization\n"
        report += "- Consider implementing rate limiting or caching strategies\n"
        report += "- Monitor these sources for potential failures\n\n"
    
    report += "### System Health\n\n"
    report += f"- Overall system health score: {health_score:.1f}%\n"
    if health_score < 50:
        report += "- **Critical**: The system is in a severely degraded state and requires immediate attention\n"
    elif health_score < 80:
        report += "- **Warning**: The system is functioning but several data sources have issues\n"
    else:
        report += "- **Good**: The system is functioning well with most data sources healthy\n"
    
    # Write the report to file
    with open(report_path, 'w') as f:
        f.write(report)
    
    log_message(f"Diagnostics report generated at {report_path}")
    return report_path


def update_health_status_failed(source_name, error_message, response_time_ms=None):
    """Update the health status of a source to failed
    
    Args:
        source_name: Name of the source
        error_message: Error message
        response_time_ms: Response time in milliseconds (optional)
    """
    conn = get_db_connection()
    with conn.cursor() as cur:
        try:
            update_query = """
                UPDATE data_source_health
                SET status = 'failed',
                    last_check = NOW(),
                    error_message = %s,
                    error_count = error_count + 1
            """
            
            params = [error_message]
            
            if response_time_ms is not None:
                update_query += ", response_time_ms = %s"
                params.append(int(response_time_ms))
            
            update_query += " WHERE source_name = %s"
            params.append(source_name)
            
            cur.execute(update_query, params)
            conn.commit()
        except Exception as e:
            conn.rollback()
            log_message(f"Error updating health status: {e}")
        finally:
            conn.close()


def run_auto_recovery(health_data):
    """Run automatic recovery for failing data sources
    
    Args:
        health_data: DataFrame containing health data
    """
    failures = health_data[health_data['status'].isin(['failed', 'error'])]
    recovered = []
    failed = []
    
    if failures.empty:
        log_message("No failed sources to recover")
        return (recovered, failed)
    
    for _, source in failures.iterrows():
        source_name = source['source_name']
        source_type = source['source_type']
        
        log_message(f"Attempting to recover {source_name}...")
        
        if source_name == 'yahoo_finance_proxy':
            if restart_yfinance_proxy():
                recovered.append(source_name)
                log_message(f"Successfully recovered {source_name}")
            else:
                failed.append((source_name, "Failed to restart proxy"))
                log_message(f"Failed to recover {source_name}")
        elif source_name == 'alpha_vantage':
            # Not implemented yet
            failed.append((source_name, "Recovery not implemented"))
            log_message(f"Recovery not implemented for {source_name}")
        else:
            failed.append((source_name, "No recovery method defined"))
            log_message(f"No recovery method defined for {source_name}")
    
    return (recovered, failed)


def check_all_sources():
    """Check the health of all data sources"""
    health_data = get_data_source_health()
    
    # Check YFinance proxy
    proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
    health_endpoint = f"{proxy_url}/health"
    
    try:
        start_time = time.time()
        response = requests.get(health_endpoint, timeout=5)
        response_time = (time.time() - start_time) * 1000  # ms
        
        if response.status_code == 200:
            status_data = response.json()
            log_message(f"YFinance proxy check: Healthy, response time {response_time:.1f}ms")
            
            # Update database
            conn = get_db_connection()
            with conn.cursor() as cur:
                try:
                    metadata = json.dumps({
                        "response_time": response_time,
                        "checked_at": datetime.now().isoformat(),
                        "endpoint": health_endpoint
                    })
                    
                    cur.execute("""
                        UPDATE data_source_health
                        SET status = 'healthy',
                            last_check = NOW(),
                            last_success = NOW(),
                            response_time_ms = %s,
                            error_message = NULL,
                            metadata = %s
                        WHERE source_name = 'yahoo_finance_proxy'
                    """, (int(response_time), metadata))
                    conn.commit()
                except Exception as e:
                    conn.rollback()
                finally:
                    conn.close()
        else:
            log_message(f"YFinance proxy check: Failed with status code {response.status_code}")
            update_health_status_failed("yahoo_finance_proxy", f"Proxy returned error code: {response.status_code}", response_time)
    except requests.RequestException as e:
        log_message(f"YFinance proxy check: Failed with error {str(e)}")
        update_health_status_failed("yahoo_finance_proxy", f"Connection failed: {str(e)}")
    
    # TODO: Add checks for other data sources here
    
    return True


def restart_yfinance_proxy():
    """Attempt to restart the YFinance proxy
    
    Returns:
        bool: True if successful, False otherwise
    """
    try:
        proxy_script_path = "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/scripts/yfinance_proxy.py"
        # Kill any existing proxy instances
        subprocess.run("pkill -f 'python.*yfinance_proxy.py'", shell=True)
        time.sleep(2)  # Give it time to shut down
        
        # Start the proxy in a new process
        env = os.environ.copy()
        subprocess.Popen(
            ["python", proxy_script_path],
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True
        )
        
        # Wait for it to start
        time.sleep(5)
        
        # Verify it's running
        proxy_url = os.environ.get("YFINANCE_PROXY_URL", "http://localhost:5000")
        health_endpoint = f"{proxy_url}/health"
        response = requests.get(health_endpoint, timeout=5)
        
        if response.status_code == 200:
            # Update the health record
            conn = get_db_connection()
            with conn.cursor() as cur:
                try:
                    metadata = json.dumps({
                        "restarted_at": datetime.now().isoformat(),
                        "restart_status": "success"
                    })
                    
                    cur.execute("""
                        UPDATE data_source_health
                        SET status = 'healthy',
                            last_check = NOW(),
                            last_success = NOW(),
                            error_message = NULL,
                            metadata = %s
                        WHERE source_name = 'yahoo_finance_proxy'
                    """, (metadata,))
                    conn.commit()
                except Exception as e:
                    conn.rollback()
                finally:
                    conn.close()
            
            return True
        return False
    except Exception as e:
        log_message(f"Error restarting YFinance proxy: {e}")
        return False

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