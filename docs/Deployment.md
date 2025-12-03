# Deployment Guide

This guide covers various deployment scenarios for the Favicon Fetcher service.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Bare Metal Deployment](#bare-metal-deployment)
- [Reverse Proxy Setup](#reverse-proxy-setup)
- [Monitoring Setup](#monitoring-setup)
- [Production Checklist](#production-checklist)

## Prerequisites

### System Requirements

- **CPU**: 1-2 cores (2+ for high traffic)
- **Memory**: 512MB minimum, 2GB recommended
- **Disk**: 10GB minimum for cache
- **Network**: Outbound HTTPS (443) and HTTP (80)

### Software Requirements

- Go 1.22+ (for building from source)
- Docker 20.10+ (for containerized deployment)
- nginx or similar (for reverse proxy)

## Docker Deployment

### Quick Start

```bash
# Build the image
docker build -t favicon-fetcher:latest .

# Run with default settings
docker run -d \
  -p 9090:9090 \
  -v favicon-cache:/app/cache \
  --name favicon-server \
  favicon-fetcher:latest

# Check health
curl http://localhost:9090/health
```

### Docker Compose

```bash
# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Production Docker Configuration

```bash
docker run -d \
  --name favicon-server \
  --restart unless-stopped \
  -p 9090:9090 \
  -v /var/cache/favicons:/app/cache \
  -e PORT=9090 \
  -e CACHE_DIR=/app/cache \
  --memory="2g" \
  --cpus="2" \
  --log-driver json-file \
  --log-opt max-size=10m \
  --log-opt max-file=3 \
  favicon-fetcher:latest \
  -addr :9090 \
  -cache-dir /app/cache \
  -cache-ttl 72h \
  -browser-max-age 24h \
  -cdn-smax-age 72h \
  -janitor-interval 30m \
  -max-cache-size-bytes 10737418240 \
  -ip-rate-limit 10 \
  -log-level info
```

## Kubernetes Deployment

### Basic Deployment

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: favicon-server
  labels:
    app: favicon-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: favicon-server
  template:
    metadata:
      labels:
        app: favicon-server
    spec:
      containers:
      - name: favicon-server
        image: favicon-fetcher:latest
        ports:
        - containerPort: 9090
          name: http
        - containerPort: 9091
          name: metrics
        env:
        - name: PORT
          value: "9090"
        - name: CACHE_DIR
          value: "/app/cache"
        args:
        - "-addr"
        - ":9090"
        - "-cache-dir"
        - "/app/cache"
        - "-cache-ttl"
        - "72h"
        - "-ip-rate-limit"
        - "10"
        - "-log-level"
        - "info"
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: cache
          mountPath: /app/cache
      volumes:
      - name: cache
        persistentVolumeClaim:
          claimName: favicon-cache-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: favicon-server
spec:
  selector:
    app: favicon-server
  ports:
  - port: 80
    targetPort: 9090
    name: http
  - port: 9091
    targetPort: 9091
    name: metrics
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: favicon-cache-pvc
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
```

### Deploy to Kubernetes

```bash
# Create namespace
kubectl create namespace favicon

# Deploy
kubectl apply -f k8s/deployment.yaml -n favicon

# Check status
kubectl get pods -n favicon
kubectl get svc -n favicon

# View logs
kubectl logs -f deployment/favicon-server -n favicon

# Scale
kubectl scale deployment favicon-server --replicas=5 -n favicon
```

### Ingress Configuration

Create `k8s/ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: favicon-ingress
  annotations:
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: favicons.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: favicon-server
            port:
              number: 80
  tls:
  - hosts:
    - favicons.example.com
    secretName: favicon-tls
```

## Bare Metal Deployment

### Build from Source

```bash
# Clone repository
git clone https://github.com/iprodev/Favicon-Fetcher.git
cd favicon-fetcher

# Build
go build -o /usr/local/bin/favicon-server ./cmd/server

# Create service user
sudo useradd -r -s /bin/false favicon

# Create directories
sudo mkdir -p /var/cache/favicons
sudo chown favicon:favicon /var/cache/favicons
```

### Systemd Service

Create `/etc/systemd/system/favicon-server.service`:

```ini
[Unit]
Description=Favicon Fetcher Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=favicon
Group=favicon
ExecStart=/usr/local/bin/favicon-server \
  -addr :9090 \
  -cache-dir /var/cache/favicons \
  -cache-ttl 72h \
  -browser-max-age 24h \
  -cdn-smax-age 72h \
  -janitor-interval 30m \
  -max-cache-size-bytes 10737418240 \
  -ip-rate-limit 10 \
  -log-level info
Restart=always
RestartSec=5s

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/cache/favicons

# Resource limits
LimitNOFILE=65536
MemoryMax=2G

[Install]
WantedBy=multi-user.target
```

### Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable on boot
sudo systemctl enable favicon-server

# Start service
sudo systemctl start favicon-server

# Check status
sudo systemctl status favicon-server

# View logs
sudo journalctl -u favicon-server -f
```

## Reverse Proxy Setup

### Nginx

Create `/etc/nginx/sites-available/favicon`:

```nginx
upstream favicon_backend {
    least_conn;
    server 127.0.0.1:9090 max_fails=3 fail_timeout=30s;
    # Add more backends for load balancing
    # server 127.0.0.1:9091 max_fails=3 fail_timeout=30s;
}

# Rate limiting
limit_req_zone $binary_remote_addr zone=favicon_limit:10m rate=10r/s;

server {
    listen 80;
    server_name favicons.example.com;

    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name favicons.example.com;

    # SSL configuration
    ssl_certificate /etc/letsencrypt/live/favicons.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/favicons.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Logging
    access_log /var/log/nginx/favicon_access.log;
    error_log /var/log/nginx/favicon_error.log;

    # Rate limiting
    limit_req zone=favicon_limit burst=20 nodelay;

    location / {
        proxy_pass http://favicon_backend;
        proxy_http_version 1.1;
        
        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
        
        # Caching
        proxy_cache_bypass $http_cache_control;
        add_header X-Cache-Status $upstream_cache_status;
    }

    location /health {
        proxy_pass http://favicon_backend;
        access_log off;
    }

    location /metrics {
        proxy_pass http://favicon_backend;
        # Restrict to internal IPs
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
    }
}
```

Enable and restart:

```bash
sudo ln -s /etc/nginx/sites-available/favicon /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Cloudflare

If using Cloudflare:

1. **DNS Setup**: Point your domain to origin server
2. **SSL/TLS**: Set to "Full (strict)"
3. **Page Rules**:
   - Cache Level: Cache Everything
   - Edge Cache TTL: 1 day
4. **Firewall Rules**:
   - Rate limiting: 100 req/min per IP
5. **Transform Rules**:
   - Add security headers

## Monitoring Setup

### Prometheus

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'favicon-server'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'
```

Start Prometheus:

```bash
docker run -d \
  -p 9091:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  --name prometheus \
  prom/prometheus
```

### Grafana Dashboard

1. Add Prometheus data source
2. Import dashboard or create custom panels:
   - Request rate
   - Response time (p50, p95, p99)
   - Cache hit rate
   - Error rate
   - Active requests

## Production Checklist

### Before Deployment

- [ ] **Build and Test**: Run all tests (`make test`)
- [ ] **Security Scan**: Check for vulnerabilities
- [ ] **Configuration Review**: Verify all settings
- [ ] **SSL Certificates**: Obtain and configure
- [ ] **Firewall Rules**: Configure ingress/egress
- [ ] **Monitoring Setup**: Prometheus + Grafana
- [ ] **Backup Strategy**: Define cache backup plan
- [ ] **Documentation**: Update runbook

### Configuration

- [ ] **Cache Settings**:
  - TTL: 72h for production
  - Max size: 10GB+
  - Janitor interval: 30m

- [ ] **Rate Limiting**:
  - IP limit: 10 req/s
  - Global limit: Based on capacity

- [ ] **Logging**:
  - Level: info (warn for minimal)
  - Rotation: Configure log rotation

- [ ] **Resources**:
  - Memory: 2GB minimum
  - CPU: 2 cores minimum
  - Disk: 20GB+ for cache

### Post-Deployment

- [ ] **Health Check**: Verify /health endpoint
- [ ] **Functional Test**: Test favicon fetching
- [ ] **Performance Test**: Load testing
- [ ] **Monitoring**: Verify metrics collection
- [ ] **Alerts**: Configure alert rules
- [ ] **Documentation**: Update deployment docs

## Troubleshooting

### Service Won't Start

```bash
# Check logs
journalctl -u favicon-server -n 50

# Common issues:
# - Port already in use: Change port
# - Permission denied: Check user permissions
# - Cache directory: Ensure it exists and is writable
```

### High Memory Usage

```bash
# Check cache size
du -sh /var/cache/favicons

# Reduce cache TTL or max size
# Add memory limits in Docker/systemd
```

### Slow Response Times

```bash
# Check metrics
curl http://localhost:9090/metrics | grep duration

# Possible causes:
# - Network latency to target sites
# - Cache full (trigger janitor)
# - CPU/memory constraints
```

### Cache Not Working

```bash
# Verify cache directory
ls -la /var/cache/favicons

# Check permissions
sudo chown -R favicon:favicon /var/cache/favicons

# Monitor cache metrics
curl http://localhost:9090/metrics | grep cache
```

## Support

For production support:
- Check logs first
- Review metrics
- Consult [SECURITY.md](../SECURITY.md) for security issues
- Open GitHub issue for bugs

---

**Last Updated**: 2025-01-01
