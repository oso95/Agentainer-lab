FROM python:3.11-slim

WORKDIR /app

RUN pip install --no-cache-dir Flask==3.0.0 google-genai gunicorn==21.2.0 python-dotenv==1.0.0 redis==4.6.0

COPY app.py .
COPY .env .

EXPOSE 8000

CMD ["gunicorn", "--bind", "0.0.0.0:8000", "--workers", "1", "--timeout", "120", "app:app"]