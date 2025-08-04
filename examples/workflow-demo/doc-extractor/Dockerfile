FROM python:3.11-slim

WORKDIR /app

# Install dependencies directly
RUN pip install --no-cache-dir \
    redis==5.0.1 \
    requests==2.31.0 \
    beautifulsoup4==4.12.2 \
    html2text==2024.2.26

# Copy application file and URLs
COPY app_multi.py ./
COPY urls.txt ./

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV REDIS_HOST=redis
ENV REDIS_PORT=6379

# Run the multi-URL processor
CMD ["python", "app_multi.py"]