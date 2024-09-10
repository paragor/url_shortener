# URL Shortener

A URL shortener service written in Go that provides a simple API for shortening URLs and redirecting users. It uses
MySQL as the database backend and integrates with Prometheus for monitoring.

## Features

- Generate short URLs for long URLs.
- Redirect users from short URLs to their corresponding long URLs.
- Health check endpoints for readiness and liveness.
- Prometheus metrics integration.
- Profiling endpoints using pprof.
- Graceful shutdown of servers.

## Prerequisites

- Go 1.16 or higher
- MySQL
- Docker (optional)

## Installation

1. Clone the repository:

```sh
git clone https://github.com/paragor/url_shortener.git
cd url_shortener
```

2. Install dependencies:

```sh
go mod tidy
```

3. Set up a MySQL database and create a DSN (Data Source Name) for connecting to it. You can use the environment
   variable `MYSQL_DSN` to configure the DSN.

## Configuration

The application can be configured using command-line flags or environment variables:

| Flag                | Environment Variable | Default                                                   | Description                             |
|---------------------|----------------------|-----------------------------------------------------------|-----------------------------------------|
| `-short-url-scheme` | `SHORT_URL_SCHEME`   | `https`                                                   | Scheme of the shortened URL.            |
| `-short-url-domain` | `SHORT_URL_DOMAIN`   |                                                           | Domain for the shortened URL.           |
| `-short-url-path`   | `SHORT_URL_PATH`     | `/`                                                       | Path prefix for the shortened URL.      |
| `-short-url-size`   | `SHORT_URL_SIZE`     | `5`                                                       | Number of characters in the short URL.  |
| `-listen-addr`      | `LISTEN_ADDR`        | `:8080`                                                   | Address to listen on for HTTP requests. |
| `-diagnostic-addr`  | `DIAGNOSTIC_ADDR`    | `:7070`                                                   | Address to listen on for diagnostics.   |
| `-mysql-dsn`        | `MYSQL_DSN`          | `user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true` | DSN for MySQL connection.               |
| `-run-migration`    |                      | `false`                                                   | Run migrations on startup.              |

## Usage

1. Build the application:

```sh
go build -o url_shortener main.go
```

2. Run the application:

```sh
./url_shortener -short-url-domain "yourdomain.com" -mysql-dsn "your-mysql-dsn"
```

You can also use environment variables to configure the application:

```sh
export MYSQL_DSN="user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true"
./url_shortener -short-url-domain "yourdomain.com"
```

3. The application will start two servers:
    - Main server for handling API requests (default `:8080`).
    - Diagnostic server for health checks and metrics (default `:7070`).

## API Endpoints

### Generate Short URL

- **Endpoint:** `/api/v1/generate_short_url`
- **Method:** `POST` or `PUT`
- **Request Body:**

```json
{
"long_url": "http://example.com",
"ttl_seconds": 3600
}
```
if ttl_seconds = 0, means link is forever exists

- **Response:**

```json
{
"short_url": "https://yourdomain.com/abc12"
}
```

### Redirect Short URL

- **Endpoint:** `/{short_url}`
- **Method:** `GET`

Redirects to the original long URL.

### Health Checks

- **Readiness Probe:** `/readyz`
- **Liveness Probe:** `/healthz`

### Prometheus Metrics

- **Endpoint:** `/metrics`

### Profiling

Profiling is enabled via `pprof` on the diagnostic server:

- **Endpoint:** `/debug/pprof/`

## Monitoring and Profiling

The application integrates with Prometheus for monitoring metrics. The `/metrics` endpoint exposes metrics that can be
scraped by Prometheus.

Additionally, Go's built-in `pprof` tool is enabled on the diagnostic server for profiling and debugging purposes.

## Running Migrations

To run database migrations, start the application with the `-run-migration` flag:

```sh
./url_shortener -run-migration
```
