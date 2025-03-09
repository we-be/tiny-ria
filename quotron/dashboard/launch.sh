#!/bin/bash

# Check if .env file exists, create from example if not
if [ ! -f .env ]; then
    echo "Creating .env file from .env.example..."
    cp .env.example .env
    echo "Please update your .env file with the correct database credentials."
fi

# Install requirements if needed
if [ ! -d "venv" ]; then
    echo "Creating virtual environment and installing dependencies..."
    python -m venv venv
    source venv/bin/activate
    pip install -r requirements.txt
else
    source venv/bin/activate
fi

# Launch the dashboard
echo "Starting the Quotron Dashboard..."
streamlit run dashboard.py