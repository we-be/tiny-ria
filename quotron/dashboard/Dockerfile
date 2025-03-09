FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    software-properties-common \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install Go for scheduler control
RUN curl -OL https://golang.org/dl/go1.21.5.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz \
    && rm go1.21.5.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin

# Copy requirements first for better caching
COPY ./dashboard/requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy the dashboard code
COPY ./dashboard /app/dashboard

# Create .env file from example if it doesn't exist
RUN if [ ! -f /app/dashboard/.env ]; then \
    cp /app/dashboard/.env.example /app/dashboard/.env; \
    fi

# Set working directory to the dashboard directory
WORKDIR /app/dashboard

# Command to run the dashboard
CMD ["streamlit", "run", "dashboard.py", "--server.port=8501", "--server.address=0.0.0.0"]