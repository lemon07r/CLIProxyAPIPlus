# CLIProxyAPI Plus (Gemini-Fix Fork)

English | [Chinese](README_CN.md)

This fork of [CLIProxyAPIPlus](https://github.com/router-for-me/CLIProxyAPIPlus) is a specialized version that resolves critical tool-calling and schema compatibility issues when using Google's Gemini v1beta API (like when trying to use claude models in openacode with the v1beta endpoint and @ai-sdk/google).

> [!IMPORTANT]
> **Current Build Status:** This fork currently pushes Docker images for **`linux/arm64`** (Apple Silicon, Raspberry Pi, etc.) every 6 hours. If you need **AMD64** support, see the [Enabling AMD64 Support](#enabling-amd64-support) section below.

## Why this fork?

The upstream repository has persistent issues when translating Claude/OpenAI tool calls into Gemini's native format. This fork implements a "Protocol Fix" that allows Google AI SDKs to work seamlessly with proxied third-party models.

### The "Gemini SDK" Compatibility Fix
This fork includes a critical patch for using Claude models (via Antigravity/other providers) with tools through the Gemini-compatible endpoint. The original implementation often caused failures in Google AI SDKs (like `@ai-sdk/google`).

**What this fork fixes:**
-   **Tool Call IDs:** Injects required unique `tool_call_id` fields (e.g., `toolu_...`) into the translation layer. Google's SDK strictly requires these to match function calls with their responses.
-   **Sequential Matching:** Implements a FIFO queue logic to correctly pair asynchronous function responses with their original calls in the conversation history.
-   **Strict Schema Cleaning:** Automatically strips unsupported JSON Schema keywords (like `$schema`, `const`, and `$ref`) from tool definitions that otherwise cause the Gemini API to return a 400 Bad Request.
-   **Role Normalization:** Ensures conversation history strictly alternates between `user` and `model` roles, preventing "invalid argument" errors common in the strict Gemini v1beta API.

---

## Enabling AMD64 Support

To build this fork for your own Intel/AMD x86_64 systems, follow these steps:

1.  **Fork this repository** to your own account.
2.  Edit `.github/workflows/build-and-push.yml` in your fork:
    *   Change `runs-on: ubuntu-24.04-arm` to `runs-on: ubuntu-latest`.
    *   Add the QEMU setup step before Docker Buildx:
        ```yaml
        - name: Set up QEMU
          uses: docker/setup-qemu-action@v3
        ```
    *   Update the `platforms` line in the build step:
        ```yaml
        platforms: linux/amd64
        ```
3.  Commit the changes. GitHub Actions will now build a 64-bit x86 image for you.

---
  
## Differences from the Mainline

- Added GitHub Copilot support (OAuth login), provided by [em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)
- Added Kiro (AWS CodeWhisperer) support (OAuth login), provided by [fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration), [Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)

## New Features (Plus Enhanced)

- **OAuth Web Authentication**: Browser-based OAuth login for Kiro with beautiful web UI
- **Rate Limiter**: Built-in request rate limiting to prevent API abuse
- **Background Token Refresh**: Automatic token refresh 10 minutes before expiration
- **Metrics & Monitoring**: Request metrics collection for monitoring and debugging
- **Device Fingerprint**: Device fingerprint generation for enhanced security
- **Cooldown Management**: Smart cooldown mechanism for API rate limits
- **Usage Checker**: Real-time usage monitoring and quota management
- **Model Converter**: Unified model name conversion across providers
- **UTF-8 Stream Processing**: Improved streaming response handling

## Kiro Authentication

### Web-based OAuth Login

Access the Kiro OAuth web interface at:

```
http://your-server:8080/v0/oauth/kiro
```

This provides a browser-based OAuth flow for Kiro (AWS CodeWhisperer) authentication with:
- AWS Builder ID login
- AWS Identity Center (IDC) login
- Token import from Kiro IDE

## Quick Deployment with Docker

### One-Command Deployment

```bash
# Create deployment directory
mkdir -p ~/cli-proxy && cd ~/cli-proxy

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    container_name: cli-proxy-api-plus
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api
      - ./logs:/CLIProxyAPI/logs
    restart: unless-stopped
EOF

# Download example config
curl -o config.yaml https://raw.githubusercontent.com/router-for-me/CLIProxyAPIPlus/main/config.example.yaml

# Pull and start
docker compose pull && docker compose up -d
```

### Configuration

Edit `config.yaml` before starting:

```yaml
# Basic configuration example
server:
  port: 8317

# Add your provider configurations here
```

### Update to Latest Version

```bash
cd ~/cli-proxy
docker compose pull && docker compose up -d
```

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
