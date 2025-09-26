# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Test
```bash
# Build the service
go build .

# Run tests (if any exist)
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

**de-mailer** is a Go microservice that sends HTML and plain text email notifications for the CyVerse Discovery Environment.

### Core Components

- **main.go**: Entry point, configuration loading, HTTP server setup on port 8080
- **api.go**: REST API handler for email requests (POST to `/`)
- **email.go**: SMTP email client implementation using gomail
- **formatMessage.go**: Template processing for HTML/text emails with DE-specific URL generation
- **error.go**: Custom HTTP error handling

### Key Design Patterns

1. **Template-based Emails**: Uses Go templates in `/templates/html/` and `/templates/text/` directories
2. **Configuration**: Uses cyverse-de/configurate for YAML config management
3. **Observability**: Integrated OpenTelemetry tracing via otelutils
4. **DE URL Integration**: Builds context-aware URLs for DE components (analyses, data, teams, etc.)

### Email Request Flow
1. POST request to `/` with EmailRequest JSON payload
2. Parse template name and values from request
3. Load appropriate template (HTML or text)
4. Format message with DE-specific URLs and context
5. Send via SMTP using configured host

### Configuration Structure
Required config keys:
- `email.smtpHost`: SMTP server address
- `email.fromAddress`: Default sender email
- `de.base`: Base URL for DE interface
- `de.data`, `de.analyses`, `de.teams`, etc.: DE component URLs