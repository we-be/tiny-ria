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
    try:
        conn = psycopg2.connect(
            host=os.environ["DB_HOST"],
            port=os.environ["DB_PORT"],
            database=os.environ["DB_NAME"],
            user=os.environ["DB_USER"],
            password=os.environ["DB_PASSWORD"]
        )
        return conn
    except Exception as e:
        st.error(f"Database connection error: {e}")
        st.info(f"Connection parameters: host={os.environ['DB_HOST']}, port={os.environ['DB_PORT']}, user={os.environ['DB_USER']}")
        return None

# Scheduler control
def get_scheduler_status():
    """Check if the scheduler is running."""
    try:
        result = subprocess.run(
            "ps aux | grep '[g]o run cmd/scheduler/main.go'", 
            shell=True, 
            capture_output=True, 
            text=True
        )
        return len(result.stdout.strip()) > 0
    except Exception as e:
        st.error(f"Error checking scheduler status: {e}")
        return False

def start_scheduler():
    """Start the scheduler process."""
    try:
        subprocess.Popen(
            "cd /home/hunter/Desktop/tiny-ria/quotron/scheduler && go run cmd/scheduler/main.go", 
            shell=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            start_new_session=True
        )
        return True
    except Exception as e:
        st.error(f"Error starting scheduler: {e}")
        return False

def stop_scheduler():
    """Stop the scheduler process."""
    try:
        subprocess.run(
            "pkill -f 'go run cmd/scheduler/main.go'", 
            shell=True, 
            capture_output=True, 
            text=True
        )
        return True
    except Exception as e:
        st.error(f"Error stopping scheduler: {e}")
        return False

def run_job(job_name):
    """Run a specific job immediately."""
    try:
        subprocess.run(
            f"cd /home/hunter/Desktop/tiny-ria/quotron/scheduler && go run cmd/scheduler/main.go -run-job={job_name}", 
            shell=True, 
            capture_output=True, 
            text=True
        )
        return True
    except Exception as e:
        st.error(f"Error running job {job_name}: {e}")
        return False

# Data fetching
def get_latest_market_indices():
    """Get the latest market indices data."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT index_name, value, change, change_percent, timestamp
                FROM (
                    SELECT *, ROW_NUMBER() OVER (PARTITION BY index_name ORDER BY timestamp DESC) as rn
                    FROM market_indices
                ) sub
                WHERE rn = 1
                ORDER BY index_name
            """)
            data = cur.fetchall()
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching market indices: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_latest_stock_quotes(limit=20):
    """Get the latest stock quotes data."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
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
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching stock quotes: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_investment_models():
    """Get all investment models."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT id, name, description, created_at
                FROM investment_models
                ORDER BY name
            """)
            data = cur.fetchall()
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching investment models: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_model_holdings(model_id):
    """Get holdings for a specific investment model."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT symbol, name, weight, sector
                FROM model_holdings
                WHERE model_id = %s
                ORDER BY weight DESC
            """, (model_id,))
            data = cur.fetchall()
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching model holdings: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_sector_allocations(model_id):
    """Get sector allocations for a specific investment model."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT sector, allocation
                FROM sector_allocations
                WHERE model_id = %s
                ORDER BY allocation DESC
            """, (model_id,))
            data = cur.fetchall()
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching sector allocations: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_data_source_health():
    """Get health status of data sources."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
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
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching data source health: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

def get_batch_statistics():
    """Get statistics for data batches."""
    conn = get_db_connection()
    if not conn:
        return pd.DataFrame()
    
    try:
        with conn.cursor(cursor_factory=RealDictCursor) as cur:
            cur.execute("""
                SELECT b.id, b.source, b.created_at, 
                       bs.record_count, bs.error_count, 
                       bs.processing_time_ms
                FROM data_batches b
                JOIN batch_statistics bs ON b.id = bs.batch_id
                ORDER BY b.created_at DESC
                LIMIT 10
            """)
            data = cur.fetchall()
            return pd.DataFrame(data) if data else pd.DataFrame()
    except Exception as e:
        st.error(f"Error fetching batch statistics: {e}")
        return pd.DataFrame()
    finally:
        conn.close()

# UI components
def render_scheduler_controls():
    """Render controls for the scheduler."""
    st.subheader("Scheduler Controls")
    
    scheduler_running = get_scheduler_status()
    status_color = "green" if scheduler_running else "red"
    status_text = "Running" if scheduler_running else "Stopped"
    
    col1, col2 = st.columns([3, 1])
    with col1:
        st.markdown(f"**Status:** <span style='color:{status_color}'>{status_text}</span>", unsafe_allow_html=True)
    
    with col2:
        if scheduler_running:
            if st.button("Stop Scheduler"):
                if stop_scheduler():
                    st.success("Scheduler stopped successfully")
                    time.sleep(1)
                    st.rerun()
        else:
            if st.button("Start Scheduler"):
                if start_scheduler():
                    st.success("Scheduler started successfully")
                    time.sleep(1)
                    st.rerun()
    
    st.divider()
    
    st.subheader("Run Individual Jobs")
    job_options = ["market_index_job", "stock_quote_job"]
    selected_job = st.selectbox("Select a job to run", job_options)
    
    if st.button(f"Run {selected_job}"):
        if run_job(selected_job):
            st.success(f"Job {selected_job} executed successfully")

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
            stocks_formatted.style.applymap(color_change, subset=['change', 'change_percent']),
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
    if st.button("Test Connection"):
        try:
            conn = psycopg2.connect(
                host=os.environ["DB_HOST"],
                port=os.environ["DB_PORT"],
                database=os.environ["DB_NAME"],
                user=os.environ["DB_USER"],
                password=os.environ["DB_PASSWORD"]
            )
            conn.close()
            st.success("✅ Connection successful!")
        except Exception as e:
            st.error(f"❌ Connection failed: {e}")
    
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
    test_conn = st.button("Test Connection")
    
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
            save_settings = st.button("Save Settings to .env")
            
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
    if st.button("Check Database Tables"):
        conn = get_db_connection()
        if conn:
            try:
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
            except Exception as e:
                st.error(f"Error checking tables: {e}")
            finally:
                conn.close()
        else:
            st.error("Cannot connect to database to check tables.")

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