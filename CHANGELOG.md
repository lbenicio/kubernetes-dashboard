# Changelog

## [1.4.7 / 1.14.4 / 1.7.10] — 2026-06-24

### Added
- **Native OAuth2/OIDC authentication** — sign in via any OIDC provider (Keycloak, Google, Okta, Authentik, PocketID, etc.)
  - Authorization Code flow with PKCE (S256)
  - Cryptographic ID token validation via JWKS (RSA + EC keys)
  - Encrypted session cookies (AES-256-GCM)
  - No Kubernetes API server configuration required — uses service account impersonation
  - Token-based login still available as fallback
- **User profile panel** — avatar, display name, and email from OIDC claims
- **Logout button** — clears OIDC session and local cookies
- **`--oidc-skip-tls-verify` flag** — trust self-signed certificates on OIDC provider
- **`--oidc-ca-bundle` flag** — custom CA bundle for OIDC provider TLS
- **Configurable OIDC claim mapping**:
  - `--oidc-username-claim` — claim for Kubernetes username (default: `email`)
  - `--oidc-groups-claim` — claim for Kubernetes groups (default: `groups`)
  - `--oidc-avatar-claim` — claim for avatar URL (default: `picture`)
  - `--oidc-name-claim` — claim for display name (default: `name`)
  - `--oidc-email-claim` — claim for email (default: `email`)
- **`--oidc-allowed-group` flag** — restrict access to users in a specific group
- **`make release APP=<app>`** — build, tag, and push a single module

### Changed
- **All namespaces** is now the default namespace filter (was `default`)
- **`kube-node-lease` and `kube-public` namespaces** are hidden from namespace dropdown and all listings
- **Pizza chart slices are clickable** — click Failed/Pending/Running to filter resource lists
- **Logout now clears OIDC sessions** in addition to local cookies
- **Login page** — centered layout, dynamic provider name on button, spinner during loading

### Added (DevOps)
- `make bump LEVEL=patch|minor|major MODULE=auth|api|web|scraper|all` — semver version bumping
- `make bump-show` — display current versions
- `make release` — build, tag (version + latest), and push all images
- `make release APP=web` — release a single module
- `make release DRY_RUN=1` — preview without executing
- `versions.env` — single source of truth for all 4 module versions
- `ARCH` auto-detection (arm64/amd64) for web builder

### Fixed
- OIDC provider discovery TLS errors on self-signed certificates (`--oidc-skip-tls-verify`)
- OAuth2 token exchange TLS errors (`--oidc-skip-tls-verify` now covers the full flow)
- OIDC callback `session expired` — `SameSite=Strict` → `Lax` for cross-site redirect compatibility
- i18n build failures from new template strings
- Login button icon rendering issues — simplified to text-only
- Login button text centering
- **Interceptor racing on page load** — checks `oidc-user` cookie directly instead of waiting for async config fetch
- **`make release` with `APP=all`** — fixed shell expansion for default case

---

## Module versions

| Module | Version | Changes since upstream |
|--------|---------|------------------------|
| dashboard-auth | **1.4.7** | OIDC flow, JWKS validation, session cookies, claim mapping, allowed groups, TLS fixes |
| dashboard-api | **1.14.4** | Impersonation auth support in client package |
| dashboard-web | **1.7.10** | Login page redesign, user profile panel, namespace defaults/filter, pizza chart clicks, status filter, interceptor |
| dashboard-scraper | 1.2.2 | _(unchanged)_ |
