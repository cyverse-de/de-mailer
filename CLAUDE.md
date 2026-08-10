# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Test
```bash
# Build the service
go build .

# Run tests
go test ./...

# Build Docker image
docker build -t de-mailer .

# Run golangci-lint (uses GitHub workflow)
golangci-lint run
```

### Running the Service
```bash
# Run locally (requires config file)
./de-mailer --config /path/to/config.yml

# Default config path expected at:
# /etc/iplant/de/emailservice.yml
```

## Architecture Overview

**de-mailer** is a Go microservice that sends HTML and plain text email notifications for the CyVerse Discovery Environment. It accepts email requests over HTTP (POST to `/` on port 8080) and consumes them from AMQP (durable queue `email_requests`, routing key `email.requests`), a role absorbed from the retired email-requests service.

### Core Components

- **main.go**: Entry point, configuration loading and validation, HTTP server on port 8080, graceful shutdown
- **api.go**: REST API handler for email requests (POST to `/`)
- **amqp.go**: AMQP consumer for email requests (cyverse-de/messaging/v12, reconnect enabled)
- **processor.go**: Transport-agnostic request processing shared by the HTTP handler and AMQP consumer
- **email.go**: SMTP email client implementation using gomail
- **formatMessage.go**: Template processing for HTML/text emails with DE-specific URL generation
- **error.go**: Custom HTTP error handling

### Key Design Patterns

1. **Template-based Emails**: Uses Go templates in `/templates/html/` and `/templates/text/` directories
2. **Configuration**: Uses cyverse-de/configurate for YAML config management; required keys are validated at startup
3. **DE URL Integration**: Builds context-aware URLs for DE components (analyses, data, teams, etc.)

### Email Request Flow
1. Request arrives via HTTP POST to `/` or as an AMQP message on the `email_requests` queue
2. Parse template name and values from the request payload
3. Load appropriate template (HTML or text)
4. Format message with DE-specific URLs and context
5. Send via SMTP using configured host

HTTP failures are reported with real status codes (400 for bad payloads/unknown templates, 500 for send failures). AMQP failures are logged with their cause and the message is acked anyway (no requeue — a failed request would almost certainly fail again).

### Configuration Structure
Required config keys (startup fails fast if any is empty):
- `email.smtpHost`: SMTP server address
- `email.fromAddress`: Default sender email
- `de.base`: Base URL for DE interface
- `amqp.uri`, `amqp.exchange.name`, `amqp.exchange.type`: AMQP connection settings
- `de.data`, `de.analyses`, `de.teams`, etc.: DE component URLs