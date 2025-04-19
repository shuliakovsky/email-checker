# email-checker

Email validation tool with MX records check, SMTP verification and disposable domains detection

```html
Key features:
    - Email format validation
    - MX records check with caching
    - SMTP server verification
    - Disposable domains detection
    - REST API mode with Swagger UI
    - Custom DNS resolver support
```
## Operation Modes
### CLI Mode
```shell
./email-checker \
  --emails "test@example.com,user@gmail.com" \
  --dns 1.1.1.1
```

### Server Mode (API)
```shell
./email-checker \
  --server \
  --port 8080 \
  --dns 8.8.8.8
```
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
Description=Email Checker Service
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/usr/local/bin/email-checker --server --port 8080 --dns 1.1.1.1
Restart=always
SyslogIdentifier=email-checker

[Install]
WantedBy=multi-user.target
```
## Metrics Output Example
```json
[
  {
    "email": "test@example.com",
    "valid": true,
    "disposable": false,
    "exists": true,
    "mx": {
      "valid": true,
      "records": [
        {"host": "mail.example.com", "priority": 10, "ttl": 3600}
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