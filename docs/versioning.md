# Versioning & Release Policy

Flotilla uses Semantic Versioning (`MAJOR.MINOR.PATCH`) across source code, container images, and downloadable binaries. This document codifies the rules and artifacts involved in each release.

---

## 1. SemVer Rules

| Component | Breakage Threshold |
|-----------|-------------------|
| **Server** (`flotilla-server`) | MAJOR increments for incompatible API/schema changes. MINOR for backward-compatible features. PATCH for bug fixes, security updates, and documentation-only releases. |
| **Agent** (`flotilla-agent`) | MAJOR increments when agent/server protocol compatibility changes. MINOR for new capabilities that remain backward compatible. PATCH for fixes and security updates. |
| **Frontend** | Distributed as part of the server; aligns with server version numbers. |

**Compatibility Matrix**

- Server `X.Y.Z` guarantees compatibility with agents `X.Y.*` (same MAJOR & MINOR).
- Agents `X.Y.Z` must connect to servers `X.Y.*` or newer patch versions.
- Cross-major connectivity is undefined and may be blocked automatically.

---

## 2. Git Tagging Strategy

- Each release is tagged with `vMAJOR.MINOR.PATCH`.
- Tags are signed (GPG) when published from the release pipeline.
- Pre-release builds (RC, beta) adopt the suffix `-rc.N`, `-beta.N`, etc. (`v1.2.0-rc.1`).
- Release branches follow the pattern `release/vMAJOR.MINOR.x` for Patch rollups.

---

## 3. Docker Images

| Image | Repository | Tags |
|-------|------------|------|
| Management Server | `ghcr.io/mikeysoft/flotilla-server` | `vMAJOR.MINOR.PATCH`, `vMAJOR.MINOR`, `vMAJOR`, `latest` |
| Agent | `ghcr.io/mikeysoft/flotilla-agent` | `vMAJOR.MINOR.PATCH`, `vMAJOR.MINOR`, `vMAJOR`, `latest` |

Multi-architecture manifests include:

- `linux/amd64`
- `linux/arm64`

Intermediate build tags use the pattern `sha-<shortsha>` for CI verification and are not pushed to `latest`.

---

## 4. Binary Artifacts

For each release tag, publish tarballs/zip archives to the GitHub Release:

```
flotilla-server_<version>_<goos>_<goarch>.tar.gz
flotilla-agent_<version>_<goos>_<goarch>.tar.gz
```

Supported combinations at launch:

- `linux_amd64`
- `linux_arm64`
- `darwin_arm64`

Archives include:

- Binary executable (`flotilla-server` or `flotilla-agent`)
- Sample configuration (`server.env.example`, `agent.yaml.example`)
- systemd unit files (`deployments/systemd/*.service`)
- `LICENSE` and `NOTICE` (if applicable)

---

## 5. Release Checklist

1. Update `CHANGELOG.md` with highlights, migration notes, and contributor acknowledgements.
2. Bump versions in `go.mod`, Dockerfiles, `Makefile`, and manifests as needed.
3. Ensure CI badges and documentation references point to the correct workflows.
4. Run `make release-deps` followed by `make release` locally to validate builds.
5. Tag the release (`git tag -s vX.Y.Z`) or rely on the release workflow to create an annotated tag.
6. Trigger the `release.yml` workflow (or create a GitHub Release) to:
   - Build multi-arch Docker images and push manifests.
   - Cross-compile binaries and attach archives to the release.
   - Update GitHub Container Registry descriptions (if configured).
7. Verify `latest` tags point to the intended version and rollback strategy is documented.

---

## 6. Changelog Management

- Maintain `CHANGELOG.md` using [Keep a Changelog](https://keepachangelog.com) style.
- Capture security fixes in a dedicated section and cross-link advisories.
- For pre-release tags, note in the changelog that features are experimental.
- After release, merge the changelog updates back into `main`.

---

## 7. Deprecation Policy

- Announce deprecated APIs at least one MINOR release before removal.
- Agents older than two MINOR releases may display "upgrade recommended" warnings.
- Document breaking changes in `docs/setup.md` and release notes.

---

## 8. Automation

Release workflows leverage GitHub Actions:

- `ci.yml` – Linting, unit tests, integration tests, security scans.
- `docker.yml` – Build & push snapshot multi-arch images (`edge`, `sha-<short>`) on `main`.
- `binaries.yml` – Cross-compile snapshot binaries on `main` and upload artifacts for testing.
- `release.yml` – Tag-triggered pipeline publishing Docker images and GitHub Release assets.

Workflows authenticate to GHCR using the repository’s `GITHUB_TOKEN`. See `.github/workflows/` for specifics.

