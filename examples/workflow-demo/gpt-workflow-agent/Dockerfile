FROM python:3.11-slim

WORKDIR /app

# Install dependencies directly
RUN pip install --no-cache-dir \
    openai==1.58.1 \
    python-dotenv==1.0.0 \
    redis==5.0.1

# Copy application and environment file
COPY app.py .
COPY .env .env

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Run the application
CMD ["python", "app.py"]