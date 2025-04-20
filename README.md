# email-checker

Email validation tool with MX records check, SMTP verification, disposable domains detection and distributed caching

```html
Key features:
    - Email format validation
    - MX records check with Redis caching
    - SMTP server verification
    - Disposable domains detection
    - REST API mode with Swagger UI
    - Custom DNS resolver support
    - Horizontal scaling with Redis
    - Configurable worker threads
```
## Operation Modes
### CLI Mode
```shell
./email-checker \
  --emails "test@example.com,user@gmail.com" \
  --dns 1.1.1.1 \
  --workers 15
```

### Server Mode (API) with Redis
```shell
./email-checker \
  --server \
  --port 8080 \
  --dns 8.8.8.8 \
  --redis redis-host:6379 \
  --redis-pass "secret" \
  --redis-db 2 \
  --workers 20
```
## Scalability Features
 - Distributed Caching: Shared Redis cache for MX records and verification results
 - TTL Management:
 - Valid emails: 30 days cache
 - Invalid emails: 24 hours cache
 - Atomic Operations: Redis-based task coordination for cluster environments
 - Auto-retry: 3 attempts for temporary SMTP errors

## API Documentation
 Swagger UI available at:
http://localhost:8080/swagger/index.html

## Build Instructions
```shell
# Build for macOS (Apple Silicon)
./build.sh darwin arm64

# Build for Linux
./build.sh linux amd64

# Show build help
./build.sh
```
## Systemd Service Example
```ini
[Unit]
Description=Email Checker Cluster Node
Requires=redis.service
After=redis.service

[Service]
ExecStart=/usr/local/bin/email-checker \
  --server \
  --port 8080 \
  --dns 1.1.1.1 \
  --redis 127.0.0.1:6379 \
  --workers 25
Restart=on-failure
SyslogIdentifier=email-checker

[Install]
WantedBy=multi-user.target
```
## Metrics Output Example
```json
[
  {
    "email": "user@company.com",
    "valid": true,
    "disposable": false,
    "exists": true,
    "cache_hit": false,
    "processing_time": "145ms",
    "mx": {
      "valid": true,
      "records": [
        {"host": "mx1.company.com", "priority": 10, "ttl": 300}
      ]
    }
  }
]
```
## Supported Protocols
SMTP ports: 25, 587, 465

DNS protocols: UDP/TCP

HTTP API: REST with JSON

### Note
For production use, consider implementing rate limiting
and authentication for the API endpoints