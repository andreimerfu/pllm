# Reference echo agent for pllm Phase 2 demo.
# Satisfies the pod contract: POST /invoke returning { "output": ..., "task_id": ... }.
FROM python:3.12-slim

WORKDIR /app

# Install deps as root, then switch to unprivileged UID to match pllm's
# AgentK8sAdapter PodSecurityContext (runAsNonRoot, runAsUser=10001).
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
 && addgroup --gid 10001 --system agent \
 && adduser --uid 10001 --system --gid 10001 --home /tmp --no-create-home agent

COPY app.py .

USER 10001:10001
EXPOSE 8000
CMD ["python", "app.py"]
