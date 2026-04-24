# Research reference agent — exercises pllm's scope layer by probing two
# LLM models (one allowed, one restricted) and reporting the result.
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
 && addgroup --gid 10001 --system agent \
 && adduser --uid 10001 --system --gid 10001 --home /tmp --no-create-home agent

COPY app.py .

USER 10001:10001
EXPOSE 8000
CMD ["python", "app.py"]
