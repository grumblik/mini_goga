# mini_goga

A lightweight Prometheus exporter for monitoring website availability and response times.

[![Go Report Card](https://goreportcard.com/badge/github.com/grumblik/mini_goga)](https://goreportcard.com/report/github.com/grumblik/mini_goga)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/grumblik/mini_goga.svg)](https://hub.docker.com/r/grumblik/mini_goga)
[![GitHub release](https://img.shields.io/github/v/release/grumblik/mini_goga)](https://github.com/grumblik/mini_goga/releases)
[![Build Status](https://github.com/grumblik/mini_goga/actions/workflows/go.yml/badge.svg)](https://github.com/grumblik/mini_goga/actions)

Originally created as a lightweight tool for Kubernetes, **mini_goga** periodically checks a list of URLs and exposes metrics about their availability, HTTP status codes, and response latency.

---

## ✨ Features

- 🚀 Simple, single-binary HTTP exporter  
- 🌐 Supports HTTP/HTTPS with custom ports  
- 📊 Prometheus-compatible metrics endpoint (`/metrics`)  
- ❤️ Health check endpoint (`/health`)  
- ⏱️ Measures response time in milliseconds  
- 🔒 Graceful connection handling (no more leaking sockets)  

---

## ⚙️ Configuration

mini_goga reads the list of URLs to monitor from a plain text file.  
Each URL **must include the scheme** (`http://` or `https://`).  

Specify the file location with the `CONFIG` environment variable.

**Example `config.cfg`:**

```
https://weurwiueyruweyriwueyriwuer.ru
http://www.google.com
http://www.google.com:80
https://flant.com:443
http://localhost:9190
http://127.0.0.1:9190/metrics
http://127.0.0.1
```

---

## 📋 Requirements

- Go 1.19+ (for building from source)
- Docker (for containerized deployment)
- Prometheus (for metrics collection)

---

## 🚀 Running

By default, the exporter listens on **port 9100**.

### Quick Start

```bash
# Create config file
echo "https://example.com" > config.cfg

# Run with Docker
docker run -d -p 127.0.0.1:9100:9100 -v $(pwd)/config.cfg:/config.cfg -e CONFIG=/config.cfg grumblik/mini_goga:latest

# Access metrics
curl http://localhost:9100/metrics
```

### Docker

```bash
docker run -d \
  -p 9100:9100 \
  -v $(pwd)/config.cfg:/config.cfg \
  -e CONFIG=/config.cfg \
  docker.io/grumblik/mini_goga:latest
```

### Binary

```bash
# Download and run
wget https://github.com/grumblik/mini_goga/releases/latest/download/mini_goga
chmod +x mini_goga
./mini_goga
```

### Docker Compose

```yaml
version: '3'
services:
  mini-goga:
    image: grumblik/mini_goga:latest
    ports:
      - "127.0.0.1:9100:9100"
    volumes:
      - ./config.cfg:/config.cfg
    environment:
      - CONFIG=/config.cfg
```

## 📊 Metrics

**Example metric output**
```
mini_goga_target_up{url="http://www.google.com:80"} 1
mini_goga_target_response_ms{url="http://www.google.com:80"} 385
mini_goga_target_status_code{url="http://www.google.com:80",code="200"} 1
mini_goga_scrape_errors_total{url="http://www.google.com:80"} 0
```

- mini_goga_target_up – 1 if the target is reachable, 0 otherwise
- mini_goga_target_response_ms – response latency in milliseconds
- mini_goga_target_status_code – one-hot gauge for the last HTTP status code
- mini_goga_scrape_errors_total – cumulative scrape errors
- mini_goga_last_success_timestamp – Unix timestamp of the last successful check

## 🔧 Environment Variables

| Variable      | Default      | Description                       |
| ------------- | ------------ | --------------------------------- |
| `CONFIG`      | `config.cfg` | Path to the file with target URLs |
| `SERVER_PORT` | `9100`       | Listening port                    |
| `INTERVAL`    | `15s`        | Interval between checks           |
| `TIMEOUT`     | `15s`        | Per-request timeout               |

## 🔧 Troubleshooting

### Common Issues

**No metrics appearing?**
- Check that your config file exists and contains valid URLs
- Verify the exporter is running: `curl http://localhost:9100/health`
- Check logs for connection errors

**High memory usage?**
- Reduce the number of targets or increase the interval
- Check for DNS resolution issues

**Connection timeouts?**
- Increase the `TIMEOUT` environment variable
- Check network connectivity to targets

## 🛠️ Building from Source

```bash
git clone https://github.com/grumblik/mini_goga.git
cd mini_goga
go build -o mini_goga .
./mini_goga
```

## 🔗 Prometheus Integration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'mini-goga'
    static_configs:
      - targets: ['mini-goga:9100']
    scrape_interval: 15s
```

## 📜 License

This project is licensed under the [MIT License](LICENSE).

✨ Simple. Minimal. Reliable. That's mini_goga.