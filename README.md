# email-checker

Email validation tool with MX records check, SMTP verification, disposable domains detection and distributed caching.

## Key Features

- ✅ **Multi-Stage Validation**
    - RFC-5322 email format verification
    - MX records validation with DNS caching
    - SMTP server availability check
    - Disposable email domain detection

- ⚡ **Performance**
    - Concurrent processing (configurable workers)
    - Redis-based caching with TTL:
        - Valid emails: 720h (30 days)
        - Invalid emails: 24h
        - MX records: 24h

- 🌐 **Distributed Architecture**
    - Horizontal scaling support
    - Redis Cluster compatibility
    - Atomic task coordination
    - Auto-recovery for failed tasks

- 📊 **Observability**
    - Detailed JSON metrics output
    - Cache statistics endpoint
    - Request logging
    - Swagger API documentation

## Operation Modes

### CLI Mode (Single Email Check)
```shell
./email-checker \
  --emails "test@example.com,user@domain.com" \
  --dns 1.1.1.1 \
  --workers 15
```

### Server Mode (REST API)
```shell
./email-checker \
  --server \
  --port 8080 \
  --dns 8.8.8.8 \
  --redis "redis-host:6379" \
  --redis-pass "your-password" \
  --workers 20
```
### Cluster Mode (Multiple Nodes)
```shell
# Node 1
./email-checker \
  --server \
  --redis "node1:6379,node2:6379,node3:6379" \
  --workers 15

# Node 2
./email-checker \
  --server \
  --redis "node1:6379,node2:6379,node3:6379" \
  --workers 15
```
### API Endpoints
 - Method	Endpoint	Description
 - POST	/tasks	Create email validation task
 - GET	/tasks/{id}	Get task status
 - GET	/tasks-results/{id}	Get paginated results
 - POST	/cache/flush	Flush all cached data
 - GET	/cache/status	Get cache statistics
 - Swagger UI: [/swagger/](https://shuliakovsky.github.io/email-checker/)
### Configuration Options
#### Core Parameters
| Flag          | Environment variable | Description        | Format  |
|---------------|----------------------|--------------------|---------|
| --dns         | DNS                  | DNS server IP      | 1.1.1.1     |
| --workers     | WORKERS              | Concurrent workers | 10            |
| --port	       | PORT                 | API server port	   | 8080               |


### Redis Configuration
| Flag      | Environment variable | Description          | default    |
|-----------|----------------------|----------------------|------------|
| --redis   | REDIS                | Redis nodes	host:port | [,host:port] |
| --redis-pass | REDIS_PASS           | Redis password       | -                     |
| --redis-db   | REDIS_DB             |API server port	      | 8080                 |

### Yaml configuration example
```yaml
#  /etc/email-checker/config.yaml
dns: 8.8.8.8
workers: 20
redis: "redis1:6379,redis2:6379"
redis-pass: "secret"
```
## Deployment
### Docker Example
```yaml
version: '3.8'

services:
  email-checker:
    image: your-registry/email-checker:latest
    environment:
      - REDIS=redis-node1:6379,redis-node2:6379
      - DNS=1.1.1.1
      - WORKERS=20
    ports:
      - "8080:8080"
    depends_on:
      - redis-cluster

  redis-cluster:
    image: redis:7.0
    command: redis-server --cluster-enabled yes
    ports:
      - "6379:6379"
```
## Systemd Service Example
```ini
[Unit]
Description=Email Checker Service
After=network.target

[Service]
ExecStart=/usr/local/bin/email-checker \
  --server \
  --port 8080 \
  --redis "redis-cluster:6379" \
  --workers 25 \
  --dns 1.1.1.1
Restart=always
User=emailchecker

[Install]
WantedBy=multi-user.target
```
## Security Recommendations
### 1. API Protection
```shell
location /api/ {
    limit_req zone=api_limit burst=10;
    auth_basic "Restricted Area";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://email-checker:8080;
}
```
### 2. Redis Security
- Enable TLS for Redis connections
- Use separate Redis user with limited permissions 
- Rotate passwords regularly
### 3. Monitoring

- Track key metrics:
- email_validation_requests_total
- cache_hit_ratio
- smtp_verification_time_ms

## Build Instructions
```shell
# Build for Apple Silicon
./build.sh darwin arm64

# Build for Linux AMD64
./build.sh linux amd64

# Show build options
./build.sh help
```

## Support Matrix
|Component| Supported Versions |
|---------|--------------------|
|Redis	| 6.2+               |
|Go	| 1.19+              |
|SMTP	| RFC 5321           |
|DNS	| UDP/TCP            |

Report Issues: https://github.com/shuliakovsky/email-checker/issues