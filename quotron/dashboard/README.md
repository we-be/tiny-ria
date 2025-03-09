# Quotron Dashboard

A Streamlit-based dashboard for monitoring and controlling the Quotron financial data system.

## Features

- **Scheduler Control**
  - Start and stop the scheduler
  - Run individual jobs on demand
  - Monitor scheduler status

- **Market Data Overview**
  - View latest market indices
  - Track top movers in the stock market
  - Color-coded indicators for price changes

- **Data Source Health**
  - Monitor health of data sources
  - Track data freshness and update frequency
  - View batch processing statistics

- **Investment Model Explorer**
  - Browse available investment models
  - Visualize sector allocations with pie charts
  - Explore model holdings with interactive charts
  - View complete holdings data

## Installation

1. Install the required packages:
   ```bash
   pip install -r requirements.txt
   ```

2. Configure database connection:
   Create a `.env` file with the following variables:
   ```
   DB_HOST=localhost
   DB_PORT=5432
   DB_NAME=quotron
   DB_USER=postgres
   DB_PASSWORD=your_password
   ```

## Running the Dashboard

```bash
# Using the launch script (recommended)
./launch.sh

# Or run directly
streamlit run dashboard.py
```

## Database Configuration

The dashboard connects to PostgreSQL with the following settings:

- Host: localhost
- Port: 5432
- Database name: quotron
- Username: quotron
- Password: quotron

You can test this connection with:
```bash
PGPASSWORD=quotron psql -U quotron -h localhost -d quotron -c "SELECT 1"
```

If needed, you can modify these settings in the Settings tab of the dashboard or by editing the `.env` file.

## Screenshots

*(Coming soon)*

## Development

The dashboard is built with:
- Streamlit for the UI framework
- Plotly for data visualization
- Pandas for data manipulation
- psycopg2 for PostgreSQL database connections
- Python subprocess for scheduler control

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request