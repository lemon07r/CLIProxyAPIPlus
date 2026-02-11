# CLIProxyAPI Plus (Custom Patches Fork)

English | [Chinese](README_CN.md)

A fork of [CLIProxyAPIPlus](https://github.com/router-for-me/CLIProxyAPIPlus) with custom patches applied on top of upstream via automated Docker builds. Patches are maintained in the [`patches/`](patches/) directory and applied in order during the Docker build process, making it easy to stay in sync with upstream while carrying custom changes.

Automated Docker builds for arm64 are available here: https://hub.docker.com/r/lemon07r/cli-proxy-api-plus/tags

> [!IMPORTANT]
> **Current Build Status:** This fork syncs with upstream and pushes Docker images for **`linux/arm64`** (Apple Silicon, Raspberry Pi, etc.) every hour. If you need **AMD64** support, see the [Enabling AMD64 Support](#enabling-amd64-support) section below.

## Patches

### 001 - Copilot Premium Requests
Updates GitHub Copilot executor headers for premium model requests. Sets `X-Initiator: agent` unconditionally, updates editor/plugin version strings, and adds per-request randomized session/machine IDs to mimic VSCode extension behavior.

### 002 - Copilot Claude Endpoint Support
Adds native Claude API support to the GitHub Copilot executor. Routes Claude models (`copilot-claude-*`) to Copilot's `/v1/messages` endpoint with proper format translation, Claude-specific usage parsing, thinking/reasoning budget normalization, and `anthropic-beta` headers. Removes unsupported `stream_options` and skips OpenAI-specific content processing for Claude requests.

### 004 - Antigravity Thinking Signature Fix
Fixes thinking signature handling in the Antigravity Gemini translator for multi-turn conversations. For Gemini models, applies the `skip_thought_signature_validator` sentinel. For Claude models, strips thinking blocks from previous assistant turns instead (Claude rejects the sentinel as an invalid signature). Also cleans up snake_case `thought_signature` fields that clients like `@ai-sdk/google` may send, preventing stale cross-provider signatures from passing through.

### 005 - Copilot Alias Prefix Stripping
Strips the `copilot-` alias prefix from model names before sending requests to GitHub Copilot's upstream API. When `oauth-model-alias` creates forked models (e.g., `copilot-gpt-5.2` from `gpt-5.2`), the alias flows through as the model name. Copilot's API only accepts the original name, so this patch updates `normalizeModel` to strip the prefix.

### Direct Source Changes (not patches)

These changes are committed directly to the fork's Go source and maintained across upstream merges:

- **Antigravity thinking translation fix**: Removed the `enableThoughtTranslate` flag from `antigravity_claude_request.go`. Upstream sets this flag to `false` when any unsigned thinking block is encountered, which globally disables thinking config for the entire request. Our fix drops unsigned blocks individually instead, so thinking remains enabled for the current model even when stale blocks from a previous model switch are present.

---

### AMD64 / Building Your Own Image

Pre-built images are **arm64 only**. If you're on AMD64 (Intel/AMD x86_64), you have two options:

**Option A: Build locally**
```bash
docker compose build
```
This uses the `build:` section already in `docker-compose.yml` to build for your native architecture.

**Option B: Fork and set up CI/CD**
1.  Fork this repository to your own account.
2.  Add `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets in your fork's GitHub settings.
3.  Edit `.github/workflows/build-and-push.yml`:
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
4.  Set `CLI_PROXY_IMAGE` in a `.env` file to point to your own Docker Hub image:
    ```
    CLI_PROXY_IMAGE=your-dockerhub-user/cli-proxy-api-plus:latest
    ```
5.  Commit the changes. GitHub Actions will build and push an AMD64 image to your registry.

---

## Original README
  
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
