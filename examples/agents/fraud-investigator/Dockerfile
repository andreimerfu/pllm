FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py pllm_sdk.py ./

# Buffered stdout would hide the step-by-step "ok / blocked / flagged"
# prints that make the demo readable.
ENV PYTHONUNBUFFERED=1

CMD ["python", "main.py"]
