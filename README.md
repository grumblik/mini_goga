# mini_goga

[![Go Report Card](https://goreportcard.com/badge/github.com/grumblik/mini_goga)](https://goreportcard.com/report/github.com/grumblik/mini_goga)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/grumblik/mini_goga.svg)](https://hub.docker.com/r/grumblik/mini_goga)
[![GitHub release](https://img.shields.io/github/v/release/grumblik/mini_goga)](https://github.com/grumblik/mini_goga/releases)
[![Build Status](https://github.com/grumblik/mini_goga/actions/workflows/go.yml/badge.svg)](https://github.com/grumblik/mini_goga/actions)

A minimal Prometheus exporter for monitoring website availability and response times.  
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

```https://weurwiueyruweyriwueyriwuer.ru
http://www.google.com:80
https://flant.com:443
http://localhost:9190
http://127.0.0.1:9190/metrics
http://127.0.0.1
```

---

## Running

By default, the exporter listens on **port 9190**.  

### Docker

```bash
docker run -d \
  -p 9190:9190 \
  -v $(pwd)/config.cfg:/config.cfg \
  -e CONFIG=/config.cfg \
  ghcr.io/grumblik/mini_goga:latest
```

** Example metric output
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

| Variable   | Default      | Description                       |
| ---------- | ------------ | --------------------------------- |
| `CONFIG`   | `config.cfg` | Path to the file with target URLs |
| `PORT`     | `9190`       | Listening port                    |
| `INTERVAL` | `15s`        | Interval between checks           |
| `TIMEOUT`  | `15s`        | Per-request timeout               |

```
git clone https://github.com/grumblik/mini_goga.git
cd mini_goga
go build -o mini_goga .
./mini_goga
```
📜 License
MIT

✨ Simple. Minimal. Reliable. That’s mini_goga.
