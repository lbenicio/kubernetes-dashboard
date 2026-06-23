# Changelog

## [1.4.3 / 1.14.3 / 1.7.3] - 2026-06-23

### Added
- **Native OAuth2/OIDC authentication** — sign in via any OIDC provider (Keycloak, Google, Okta, Authentik, etc.)
  - Authorization Code flow with PKCE (S256)
  - Cryptographic ID token validation via JWKS (RSA + EC keys)
  - Encrypted session cookies (AES-256-GCM)
  - No Kubernetes API server configuration required — uses service account impersonation
  - Token-based login still available as fallback
- **User profile panel** — avatar, display name, and email from OIDC claims
- **Logout button** — clears OIDC session and local cookies
- **`--oidc-skip-tls-verify` flag** — trust self-signed certificates on OIDC provider

### Changed
- **All namespaces** is now the default namespace filter (was `default`)
- **`kube-node-lease` and `kube-public` namespaces** are hidden from namespace dropdown and all listings
- **Pizza chart slices are clickable** — click Failed/Pending/Running to filter resource lists
- **Logout now clears OIDC sessions** in addition to local cookies

### Added (DevOps)
- `make bump LEVEL=patch|minor|major MODULE=auth|api|web|scraper|all` — semver version bumping
- `make bump-show` — display current versions
- `make release` — build, tag (version + latest), and push all images in one command
- `make release DRY_RUN=1` — preview without executing
- `versions.env` — single source of truth for all 4 module versions
- `ARCH` auto-detection (arm64/amd64) for web builder

### Fixed
- OIDC provider discovery TLS errors on self-signed certificates (`--oidc-skip-tls-verify`)
- i18n build failures from new template strings

---

## Image versions

| Module | Version | Changes |
|--------|---------|---------|
| dashboard-auth | **1.4.3** | OIDC routes, JWKS validation, session management, user info extraction, TLS flags |
| dashboard-api | **1.14.3** | Impersonation auth support in client package |
| dashboard-web | **1.7.3** | Login page, user panel, namespace defaults/filter, pizza chart clicks, status filter, interceptor |
| dashboard-scraper | 1.2.2 | _(unchanged)_ |
