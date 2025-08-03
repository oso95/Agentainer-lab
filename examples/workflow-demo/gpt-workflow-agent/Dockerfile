FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application and environment file
COPY app.py .
COPY .env .env

# Environment variables (set defaults, can be overridden)
ENV PYTHONUNBUFFERED=1
ENV PORT=8080
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Expose port
EXPOSE ${PORT}

# Run the application
CMD ["python", "app.py"]