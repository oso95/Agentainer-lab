FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application files
COPY app.py app_web.py app_multi.py ./

# Copy URLs file if it exists
COPY urls.txt* ./

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV PORT=8080
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Expose port
EXPOSE ${PORT}

# Default to multi-URL version for workflow demo
CMD ["python", "app_multi.py"]