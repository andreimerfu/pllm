# Deployment Guide

PLLM supports multiple deployment methods for different environments.

## Docker Deployment

### Docker Compose (Recommended)

The complete stack includes PLLM, PostgreSQL, Redis, Dex, Grafana, Prometheus, and Jaeger:

```bash
# Clone repository
git clone https://github.com/andreimerfu/pllm.git
cd pllm

# Set environment variables
cp .env.example .env
# Edit .env with your API keys and configuration

# Start full stack
docker-compose up -d
```

**Services started:**
- **PLLM Gateway**: `localhost:8080` (API), `localhost:8081` (Admin), `localhost:9090` (Metrics)
- **PostgreSQL**: `localhost:5432`
- **Redis**: `localhost:6380`
- **Dex OIDC**: `localhost:5556`
- **Grafana**: `localhost:3001` (admin/admin)
- **Prometheus**: `localhost:9091`
- **Jaeger**: `localhost:16686`

### Single Container

Run just PLLM in a container:

```bash
# Build image
docker build -t pllm .

# Run with external database
docker run -d \
  --name pllm-gateway \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/pllm" \
  -e REDIS_URL="redis://host:6379" \
  -e OPENAI_API_KEY="sk-your-key" \
  -e PLLM_MASTER_KEY="sk-master-key" \
  -v $(pwd)/config.yaml:/app/config.yaml \
  pllm
```

## Kubernetes

### Helm Chart

PLLM includes a production-ready Helm chart:

```bash
cd deploy/helm

# Install with default values
helm install pllm ./pllm

# Or customize values
helm install pllm ./pllm -f values-production.yaml

# Upgrade deployment
helm upgrade pllm ./pllm
```

**Chart includes:**
- PLLM deployment with HPA
- PostgreSQL (with persistence)
- Redis cluster
- Ingress configuration
- ServiceMonitor for Prometheus
- ConfigMaps and Secrets

### Manual Kubernetes

Example minimal deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pllm-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: pllm-gateway
  template:
    metadata:
      labels:
        app: pllm-gateway
    spec:
      containers:
      - name: pllm
        image: amerfu/pllm:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: pllm-secrets
              key: database-url
        - name: REDIS_URL
          value: "redis://redis-service:6379"
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: pllm-secrets
              key: openai-api-key
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: pllm-service
spec:
  selector:
    app: pllm-gateway
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

## Binary Deployment

### Build from Source

```bash
# Clone and build
git clone https://github.com/andreimerfu/pllm.git
cd pllm

# Install dependencies
make deps

# Build binary
make build

# The binary is now at ./bin/pllm-server
./bin/pllm-server --help
```

### Download Pre-built Binary

```bash
# Download latest release
wget https://github.com/amerfu/pllm/releases/latest/download/pllm-linux-amd64
chmod +x pllm-linux-amd64
mv pllm-linux-amd64 /usr/local/bin/pllm

# Run
pllm --config /etc/pllm/config.yaml
```

### Systemd Service

Create `/etc/systemd/system/pllm.service`:

```ini
[Unit]
Description=PLLM Gateway
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=pllm
Group=pllm
WorkingDirectory=/opt/pllm
ExecStart=/usr/local/bin/pllm --config /etc/pllm/config.yaml
Restart=always
RestartSec=5
Environment=LOG_LEVEL=info

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/pllm
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable pllm
sudo systemctl start pllm
sudo systemctl status pllm
```

## Production Configuration

### Rate Limiting Behavior

**Important**: PLLM's rate limiting is designed to protect API endpoints while allowing free access to:

- **Documentation**: `/docs` (including all VitePress assets)
- **Admin UI**: `/ui` (including all React app assets)
- **Health checks**: `/health`, `/ready`, `/metrics`
- **API documentation**: `/swagger`
- **Static assets**: CSS, JS, images, fonts

This ensures that users can always access documentation and the admin interface even when API rate limits are hit.

### Environment Variables

**Required:**
```bash
# Database (required for auth)
DATABASE_URL=postgres://pllm:password@localhost:5432/pllm?sslmode=require

# Redis (required for caching/budget)
REDIS_URL=redis://localhost:6379

# At least one LLM provider
OPENAI_API_KEY=sk-your-openai-key

# Security
PLLM_MASTER_KEY=sk-secure-master-key-32-chars-min
JWT_SECRET_KEY=your-very-secure-jwt-signing-key
```

**Recommended:**
```bash
# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Monitoring
ENABLE_METRICS=true
ENABLE_TRACING=true
JAEGER_ENDPOINT=http://jaeger:14268/api/traces

# Authentication (production)
DEX_ENABLED=true
DEX_ISSUER=https://auth.yourdomain.com/dex
DEX_CLIENT_ID=pllm-production
DEX_CLIENT_SECRET=secure-client-secret
DEX_REDIRECT_URL=https://pllm.yourdomain.com/auth/callback
```

### Load Balancing

#### Provider Load Balancing

Configure multiple API keys for provider redundancy:

```bash
# Multiple OpenAI keys
OPENAI_API_KEY=sk-primary-key
OPENAI_API_KEY_1=sk-backup-key-1
OPENAI_API_KEY_2=sk-backup-key-2

# Multiple providers
ANTHROPIC_API_KEY_1=sk-ant-key
AZURE_API_KEY_EAST=azure-east-key
GROK_API_KEY_1=grok-key
```

#### Application Load Balancing

Use a load balancer in front of multiple PLLM instances:

**nginx configuration:**
```nginx
upstream pllm_backend {
    server pllm-1:8080 weight=1 max_fails=3 fail_timeout=30s;
    server pllm-2:8080 weight=1 max_fails=3 fail_timeout=30s;
    server pllm-3:8080 weight=1 max_fails=3 fail_timeout=30s;

    # Health check
    keepalive 32;
}

server {
    listen 80;
    server_name pllm.yourdomain.com;

    location / {
        proxy_pass http://pllm_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # For streaming responses
        proxy_buffering off;
        proxy_cache off;

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://pllm_backend/health;
        access_log off;
    }
}
```

### Security

#### TLS/HTTPS

**Nginx with Let's Encrypt:**
```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Get certificate
sudo certbot --nginx -d pllm.yourdomain.com

# Auto-renewal
sudo crontab -e
0 12 * * * /usr/bin/certbot renew --quiet
```

#### Firewall

Configure firewall rules:
```bash
# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow SSH (be careful!)
sudo ufw allow 22/tcp

# Block direct access to PLLM ports
sudo ufw deny 8080/tcp
sudo ufw deny 8081/tcp

# Enable firewall
sudo ufw enable
```

#### Database Security

```bash
# PostgreSQL security
sudo -u postgres psql
ALTER USER pllm WITH PASSWORD 'secure-random-password';
CREATE DATABASE pllm OWNER pllm;
```

### Monitoring & Observability

#### Prometheus Configuration

`/etc/prometheus/prometheus.yml`:
```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'pllm'
    static_configs:
      - targets: ['pllm-1:9090', 'pllm-2:9090', 'pllm-3:9090']
    scrape_interval: 10s
    metrics_path: /metrics

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']

  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']

  - job_name: 'node'
    static_configs:
      - targets: ['node-exporter:9100']
```

#### Grafana Dashboards

PLLM includes pre-built Grafana dashboards:
- **System Metrics**: `/config/grafana-dashboards/pllm-system-metrics.json`
- **Performance**: `/config/grafana-dashboards/pllm-performance.json`

Import these dashboards in Grafana UI.

#### Log Aggregation

**Using Fluentd/Fluent Bit:**
```yaml
# fluent-bit.conf
[INPUT]
    Name tail
    Path /var/log/pllm/*.log
    Parser json
    Tag pllm.*

[OUTPUT]
    Name elasticsearch
    Match pllm.*
    Host elasticsearch
    Port 9200
    Index pllm-logs
```

### High Availability

#### Multi-Region Deployment

Deploy PLLM in multiple regions with shared Redis cluster:

```yaml
# Region 1: US-East
- PLLM instances: 3
- PostgreSQL: Primary
- Redis: Master

# Region 2: EU-West
- PLLM instances: 3
- PostgreSQL: Read replica
- Redis: Replica

# Global Load Balancer
# Route based on geography
```

#### Database HA

**PostgreSQL with streaming replication:**
```bash
# Primary server
postgresql.conf:
  wal_level = replica
  max_wal_senders = 3
  wal_keep_segments = 64

# Replica server
recovery.conf:
  standby_mode = 'on'
  primary_conninfo = 'host=primary-db port=5432 user=replicator'
```

**Redis Cluster:**
```bash
# 3 masters + 3 replicas minimum
redis-cluster create \
  host1:7000 host2:7000 host3:7000 \
  host1:7001 host2:7001 host3:7001 \
  --cluster-replicas 1
```

### Backup & Disaster Recovery

#### Database Backup

```bash
#!/bin/bash
# Daily PostgreSQL backup
pg_dump -h localhost -U pllm -d pllm \
  | gzip > /backups/pllm-$(date +%Y%m%d).sql.gz

# Retain 30 days
find /backups -name "pllm-*.sql.gz" -mtime +30 -delete
```

#### Configuration Backup

```bash
#!/bin/bash
# Backup configurations
tar -czf /backups/pllm-config-$(date +%Y%m%d).tar.gz \
  /etc/pllm/ \
  /opt/pllm/config/ \
  /etc/systemd/system/pllm.service
```

### Troubleshooting

#### Common Issues

**Connection errors:**
```bash
# Check service status
sudo systemctl status pllm

# Check logs
sudo journalctl -u pllm -f

# Test database connection
pg_isready -h localhost -p 5432 -U pllm

# Test Redis connection
redis-cli -h localhost -p 6379 ping
```

**Performance issues:**
```bash
# Check metrics
curl http://localhost:9090/metrics | grep pllm

# Monitor resources
htop
iotop -ao
```

**Authentication problems:**
```bash
# Verify Dex connectivity
curl http://localhost:5556/dex/.well-known/openid_configuration

# Check JWT token validity
echo "your.jwt.token" | base64 -d
```

#### Debug Mode

Enable debug logging:
```bash
LOG_LEVEL=debug pllm --config config.yaml
```

Or via environment:
```bash
export LOG_LEVEL=debug
sudo systemctl restart pllm
```
