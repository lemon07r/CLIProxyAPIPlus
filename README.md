# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This repository is `lemon07r/CLIProxyAPIPlus`, a fork of the upstream [CLIProxyAPI Plus](https://github.com/router-for-me/CLIProxyAPIPlus) branch. The fork stays close to upstream and carries a small patch stack for Copilot and Antigravity behavior that is applied during Docker builds.

Published images for this fork are available on Docker Hub as [`lemon07r/cli-proxy-api-plus`](https://hub.docker.com/r/lemon07r/cli-proxy-api-plus). On pushes to `main`, the fork's `build-and-push.yml` workflow syncs upstream, builds an ARM64 image, and pushes `latest`. Tagged builds use `docker-image.yml` for multi-arch releases.

The current fork-specific stack is intentionally small: nine numbered patches grouped around Copilot routing/fingerprinting, Antigravity compatibility/fingerprinting, and one Claude streaming fix. When a later tweak is really just an extension of an existing feature, the preferred maintenance move is to fold it back into that patch instead of piling on more tiny follow-up patches.

## What Is Different In This Fork

- Custom behavior lives in [`patches/`](patches/), not as long-lived source edits.
- The Docker build applies patch files in lexical order during `docker build`.
- The goal is to stay mergeable with upstream while keeping the fork-specific behavior isolated.

If you change Go source directly and commit that source change instead of updating the matching patch file, the next upstream sync will overwrite your work. For fork maintenance, treat the patch files as the real source of truth.

## How The Patch Build Works

1. The repo tracks upstream normally.
2. Fork changes live as numbered patch files in [`patches/`](patches/).
3. [`Dockerfile`](Dockerfile) copies the repo, then applies `patches/*.patch` in sorted order.
4. The patched tree is compiled into the final `CLIProxyAPIPlus` binary.

That means the important workflow is:

```bash
# 1. Apply earlier patches to get the right baseline
git apply patches/001-unlimited-copilot-headers.patch
# ...apply through the patch before the one you are editing

# 2. Edit the source file temporarily

# 3. Generate/update the patch file

# 4. Revert temporary source edits before committing
git checkout -- internal/ sdk/
```

## Patch Stack

| Patch | Purpose |
|---|---|
| `001-unlimited-copilot-headers.patch` | Spoofs Copilot/VS Code headers and sets the Copilot header baseline used by the fork. |
| `002-copilot-claude-endpoint.patch` | Routes Claude models to Copilot's `/v1/messages`, uses `/responses` for GPT-5.3/Codex-style models, strips the `copilot-` prefix, and improves Copilot Claude streaming/thinking behavior. |
| `003-antigravity-claude-thinking-signature-fix.patch` | Fixes Claude thinking signature handling for Antigravity translators. |
| `004-antigravity-assistant-prefill-fix.patch` | Rewrites Claude-model assistant prefill only in the Antigravity Gemini translator path. |
| `005-antigravity-merge-consecutive-turns.patch` | Merges consecutive same-role turns for Antigravity backends. |
| `006-antigravity-anti-fingerprinting.patch` | Adds per-account Antigravity fingerprinting on top of upstream version updates. |
| `007-copilot-responses-vision-detection.patch` | Extends Copilot vision detection to Responses API `input[]` image items. |
| `008-streaming-tool-call-deltas.patch` | Streams Claude tool call argument deltas incrementally in the Claude-to-OpenAI translator. |
| `009-copilot-anti-fingerprinting.patch` | Adds per-account Copilot header diversity, persistent MachineId/SessionId behavior, conversation-aware warm-session billing, compaction-stable warm keys, and cold-session startup reservation. |

## Patch Layout

- `001`, `002`, `007`, `009`: Copilot executor behavior, endpoint routing, and fingerprinting/session policy.
- `003`, `004`, `005`, `006`: Antigravity request translation fixes plus anti-fingerprinting behavior.
- `008`: Claude streaming translation fix.

This split is deliberate. It keeps unrelated providers separated, but avoids stacking multiple tiny follow-up patches on the exact same Copilot session logic.

## Common Workflows

### Run The Prebuilt Image

```bash
./docker-build.sh
# choose option 1
```

Or directly:

```bash
docker compose up -d --remove-orphans --no-build
```

### Build From Source Locally

```bash
./docker-build.sh
# choose option 2
```

Or directly:

```bash
docker build -t cliproxy-api-plus:local .
docker compose up -d --remove-orphans --pull never
```

### Build An x86/amd64 Image On ARM

If you are on an ARM VPS but need to test the x86 build locally:

```bash
docker buildx build --platform linux/amd64 -t cliproxy-api-plus:amd64 --load .
```

If you want to publish that image instead of loading it into the local Docker daemon:

```bash
docker buildx build --platform linux/amd64 -t yourname/cli-proxy-api-plus:amd64 --push .
```

### Test The Patch Chain Cleanly

```bash
git checkout -- internal/ sdk/

for patch in patches/*.patch; do
  git apply "$patch"
done

docker run --rm -v "$PWD:/src" -w /src golang:1.26-alpine \
  sh -lc '/usr/local/go/bin/go test ./internal/runtime/executor -run "TestApplyHeaders_XInitiator|TestApplyHeaders_GitHubAPIVersion|TestApplyHeaders_OpenAIIntentValue"'

git checkout -- internal/ sdk/
```

### Push A Fork Change Live

```bash
git add patches/ README.md
git commit -m "your message"
git push origin main
```

After that:

- `build-and-push.yml` builds and publishes the ARM64 Docker image
- tagged builds can use `docker-image.yml` for multi-arch releases
- if your server uses watchtower, it will pull the new `latest` automatically
- if not, pull and restart manually with `docker compose pull && docker compose up -d`

## Repo Layout Notes

- [`docker-compose.yml`](docker-compose.yml) is the repo-local compose file for building or running this repo directly.
- [`docker-build.sh`](docker-build.sh) is the easiest beginner-friendly entry point for local builds.
- [`config.example.yaml`](config.example.yaml) is the main config reference.
- Production deployments often keep a separate runtime directory that mounts `config.yaml`, `auths/`, and `logs/` into the container instead of running straight from the git checkout.

All third-party provider support is maintained by community contributors; upstream CLIProxyAPI does not provide technical support for fork-specific changes. If you need help with this fork, contact the corresponding fork maintainer.

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
