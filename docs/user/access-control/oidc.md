# OpenID Connect login

Kubernetes Dashboard can sign users in through an OpenID Connect provider using the OAuth 2.0 authorization code flow with PKCE. The Auth service performs provider discovery, exchanges the authorization code, and verifies the ID token's signature, issuer, audience, expiry, subject, and nonce. Provider tokens stay in encrypted, `HttpOnly` session cookies. Cookies use `SameSite=Lax` for the standard query callback, or `SameSite=None; Secure` when the provider supports `form_post`.

## Configure the identity provider

Register Dashboard as a confidential web client at the identity provider, or as a public client only when the provider supports public clients with PKCE. Register this exact callback URL, replacing the origin with the public HTTPS address of the Dashboard:

```text
https://dashboard.example.com/api/v1/oidc/callback
```

The Auth service must be able to reach the issuer's HTTPS discovery document, token endpoint, JWKS endpoint, and (if used) UserInfo endpoint. The public Dashboard URL and redirect URL must use HTTPS. HTTP is accepted only for issuer and redirect URLs on loopback addresses for local development.

Configure these environment variables on the Auth service:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `OIDC_ISSUER_URL` | Yes | — | Exact issuer URL from the provider's discovery document. |
| `OIDC_CLIENT_ID` | Yes | — | Registered client ID. |
| `OIDC_CLIENT_SECRET` | For confidential clients | — | Confidential client secret. Store it as a secret. Leave unset only for a provider-registered public client that supports PKCE. |
| `OIDC_REDIRECT_URL` | Yes | — | Exact callback URL registered above. |
| `OIDC_SCOPES` | No | `openid profile email` | Space-separated scopes requested during login. `openid` is always included. |
| `OIDC_GROUPS_CLAIM` | No | `groups` | Claim containing group names. |
| `OIDC_PICTURE_CLAIM` | No | `picture` | Claim containing the user's profile image URL. |

Set `DASHBOARD_SESSION_KEY` to the same base64-encoded 32-byte secret on both Auth and API services. For example, generate a key with `openssl rand -base64 32`, then store it in a secret manager and mount it into both services. Changing the key invalidates existing browser sessions. Keep `DASHBOARD_SESSION_COOKIE_SECURE=true` for public deployments. The development Compose file defaults it to `false` for local HTTP use.

When OIDC is configured, the login page initially offers only **Sign in with SSO**, with a small link to reveal bearer-token login. A provider must return an ID token that the Kubernetes API server accepts. If the provider issues refresh tokens and you want Dashboard to refresh sessions, request the provider's supported offline scope (commonly `offline_access`) in `OIDC_SCOPES`.

## Configure Kubernetes authentication and authorization

Dashboard forwards the verified ID token to the Kubernetes API server. Configure the API server to trust the same issuer and client audience, and map the issuer's username and group claims. The group claim used for Kubernetes authorization must be included in the ID token; a group value returned only by UserInfo can be displayed in Dashboard but is not part of the bearer credential sent to Kubernetes. For example, the Kubernetes API server's OIDC authentication settings need to agree with the issuer URL, client ID, username claim, and `OIDC_GROUPS_CLAIM` used here. See the [Kubernetes OIDC authentication documentation](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens).

Dashboard does not grant permissions from the groups displayed in the user menu. Kubernetes RBAC evaluates the groups from the accepted ID token and decides which resources the user can access. Create RoleBindings or ClusterRoleBindings for those groups in Kubernetes.

The picture claim is shown as the account avatar only when it contains an HTTPS URL. It is fetched by the browser directly from that image host with referrer information disabled.

## Other login modes

Bearer-token login remains available as a fallback link when OIDC is configured. Without OIDC, the existing token login page remains the default.

Set `DASHBOARD_ANONYMOUS_ADMIN=true` on both Auth and API to skip login and use the API service's configured Kubernetes credentials for requests without a user token. Anonymous mode cannot be enabled together with OIDC. Those credentials determine the actual permissions; bind them to `cluster-admin` only if that access is intended. Every network-reachable visitor can use those permissions. Keep this mode off for any deployment that is reachable by untrusted users.

Authentication is removed from Dashboard when the user signs out. Provider-wide single sign-out depends on the identity provider and is not implied by local Dashboard sign-out.
