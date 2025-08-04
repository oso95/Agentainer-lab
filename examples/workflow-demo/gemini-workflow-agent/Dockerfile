FROM python:3.11-slim

WORKDIR /app

# Install dependencies directly
RUN pip install --no-cache-dir \
    redis==5.0.1 \
    google-generativeai==0.8.3 \
    python-dotenv==1.0.0

# Copy application and environment file
COPY app.py .
COPY .env .env

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Run the application
CMD ["python", "app.py"]