# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-12-03

### Added

- **Core Features**
  - HTTP service for fetching and serving website favicons
  - Smart favicon discovery from HTML `<link>` tags
  - Support for Apple Touch Icons
  - Fallback to `/favicon.ico`
  - Configurable output size (16-256 pixels)

- **Input Format Support**
  - ICO (with multi-resolution support)
  - SVG (high-quality rasterization)
  - PNG
  - JPEG
  - GIF
  - WebP
  - AVIF
  - BMP

- **Output Format Support**
  - PNG (default)
  - WebP (via Accept header)
  - AVIF (via Accept header, best compression)

- **Caching**
  - 3-tier cache system (original, resized, fallback)
  - Configurable TTL
  - ETag and Last-Modified support
  - Conditional requests (304 Not Modified)
  - Automatic cache cleanup (janitor)
  - Maximum cache size limit

- **Security**
  - SSRF protection
  - Private IP blocking (RFC 1918)
  - Localhost/loopback blocking
  - DNS rebinding prevention
  - Scheme validation (HTTP/HTTPS only)
  - Redirect limits (max 8)
  - Request timeout (12s)
  - Size limits (4MB images, 1MB HTML)

- **Performance**
  - Request deduplication (singleflight pattern)
  - Token bucket rate limiting (global and per-IP)
  - HTTP/2 support
  - Gzip response support

- **Observability**
  - Prometheus-compatible metrics endpoint
  - Structured logging with configurable levels
  - Health check endpoint

- **Deployment**
  - Docker support with multi-arch images (amd64, arm64)
  - Docker Compose configuration
  - Graceful shutdown
  - GitHub Actions CI/CD
  - Pre-built binaries for Linux, macOS, Windows

### Technical Details

- Built with Go 1.22+
- Uses tdewolff/canvas for SVG rendering
- Uses gen2brain/avif for AVIF support
- Uses kolesa-team/go-webp for WebP support
- Zero external runtime dependencies (static binary)

[1.0.0]: https://github.com/iprodev/Favicon-Fetcher/releases/tag/v1.0.0
