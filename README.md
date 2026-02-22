# CLIProxyAPI Plus (Custom Patches Fork)

English | [Chinese](README_CN.md)

A fork of [CLIProxyAPIPlus](https://github.com/router-for-me/CLIProxyAPIPlus) with custom patches applied on top of upstream via automated Docker builds. Patches are maintained in the [`patches/`](patches/) directory and applied in order during the Docker build process, making it easy to stay in sync with upstream while carrying custom changes.

Automated Docker builds for arm64 are available here: https://hub.docker.com/r/lemon07r/cli-proxy-api-plus/tags

> [!IMPORTANT]
> **Current Build Status:** This fork syncs with upstream and pushes Docker images for **`linux/arm64`** (Apple Silicon, Raspberry Pi, etc.) every hour. If you need **AMD64** support, see the [Enabling AMD64 Support](#enabling-amd64-support) section below.

## Patches

### 001 - Copilot Premium Requests
Updates GitHub Copilot executor headers for premium model requests. Sets `X-Initiator: agent` unconditionally, updates editor/plugin version strings, and adds per-request randomized session/machine IDs to mimic VSCode extension behavior.

### 002 - Copilot Claude & GPT-5.3 Endpoint Support
Adds native Claude API and GPT-5.3 `/responses` endpoint support to the GitHub Copilot executor. Routes Claude models (`copilot-claude-*`) to Copilot's `/v1/messages` endpoint with proper format translation, Claude-specific usage parsing, thinking/reasoning budget normalization, and `anthropic-beta` headers. Routes GPT-5.3 models to the `/responses` endpoint with codex format translation, including custom responses→Claude stream/non-stream translators for clients that speak Claude format. Strips the `copilot-` alias prefix from model names before sending requests upstream. Adds SSE passthrough newline preservation for claude→claude streaming when no response translator is registered. Includes smarter streaming usage parsing with fallback from OpenAI to Claude format when `TotalTokens` is zero.

### 003 - Antigravity Thinking Signature Fix
Fixes thinking signature handling in the Antigravity Gemini translator for multi-turn conversations. For Gemini models, applies the `skip_thought_signature_validator` sentinel. For Claude models, strips thinking blocks from previous assistant turns instead (Claude rejects the sentinel as an invalid signature). Also cleans up snake_case `thought_signature` fields that clients like `@ai-sdk/google` may send, preventing stale cross-provider signatures from passing through.

### 004 - Antigravity Assistant Prefill Fix
Handles Claude assistant message prefill for the Antigravity backend. Claude clients (opencode, Claude Code, etc.) send a trailing assistant message with partial content to guide the model's response — a Claude-specific feature the Antigravity/Gemini API rejects. This patch detects trailing model messages (that aren't tool-use turns), extracts the prefill text, and replaces them with a synthetic user message (`"Continue from: <prefill>"`) to preserve the intent. Scoped to Claude models only — native Gemini models are unaffected.

### 005 - Antigravity Merge Consecutive Turns
Merges consecutive same-role turns in the Antigravity Gemini translator before sending requests to the Gemini API. The Gemini API requires strict `user` → `model` → `user` → `model` alternation — consecutive turns with the same role cause an `INVALID_ARGUMENT` error. This patch detects adjacent turns sharing a role and combines their `parts` arrays into a single turn, preserving all content while satisfying the API constraint. Commonly triggered by long multi-turn conversations where the OpenAI/Claude → Gemini translation produces back-to-back model messages.

### 006 - Antigravity Anti-Fingerprinting
Comprehensive anti-fingerprinting for the Antigravity executor — goes well beyond what other proxy solutions offer by making each auth account look like a distinct, real Antigravity IDE installation rather than a single proxy instance multiplexing requests.

**Why this matters:** Without these fixes, all requests from a proxy share identical session IDs, user-agent strings, and project names — trivially detectable patterns that no legitimate user would produce. Most proxy solutions either ignore this entirely or use a single hardcoded user-agent for all traffic.

**Session ID:** Salts the session ID with each auth token's unique ID so that different accounts never share the same session, preventing cross-account correlation. Fixes the format to match real Antigravity traffic (`-{uuid}:{model}:{project}:seed-{hex16}` instead of the upstream bare numeric string).

**User-Agent:** Each auth account gets its own deterministic, cached user-agent string with a realistic Antigravity version and platform combination (e.g., `antigravity/1.18.3 darwin/arm64`). Versions are fetched dynamically from the [Antigravity auto-updater API](https://antigravity-auto-updater-974169037036.us-central1.run.app) every 12 hours so the proxy always advertises the current stable release — just like real users who auto-update. Per-account version tracking ensures accounts never downgrade to an older version if a fetch fails and the static fallback pool is used. Falls back gracefully to a static version pool (`1.16.5`–`1.18.3`) if the API is unreachable.

**Project ID:** Expands the fallback random project name word pools from 5×5 (25 combinations) to 30×30 (900 combinations), dramatically reducing the chance of two accounts generating the same project name.

---

## Example Configs

- **[`config.example.custom.yaml`](config.example.custom.yaml)** -- Proxy config with model aliases for Qwen, Codex, Copilot, Antigravity, and Kimi providers. Copy to `config.yaml` and fill in your secrets.
- **[`example.opencode.json`](example.opencode.json)** -- [opencode](https://opencode.ai) client config with all providers pre-configured. Copy to `~/.config/opencode/opencode.json` and update the `baseURL` and `apiKey` fields.

> [!TIP]
> Using a different AI coding agent (Cline, Droid, Kilo, etc.)? You can feed `example.opencode.json` to an AI and ask it to convert the provider/model definitions into your agent's config format. The model IDs, context limits, and endpoint URLs are the same regardless of client.

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
