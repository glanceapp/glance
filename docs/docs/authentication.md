# Authentication

Dynacat supports two authentication methods: username/password and OIDC (Single Sign-On). Both methods can be active at the same time.

## Password Authentication

Protect your dashboard with username and password login. Configure via the top-level `auth` property:

```yaml
auth:
  secret-key: # generate with: ./dynacat secret:make
  users:
    admin:
      password: mysecretpassword
    alice:
      password: anotherpassword
```

To generate a secret key:

```sh
./dynacat secret:make
```

Or with Docker:

```sh
docker run --rm Panonim/dynacat secret:make
```

### Hashed passwords

Avoid storing plain passwords in your config by hashing them first:

```sh
./dynacat password:hash mysecretpassword
```

Or with Docker:

```sh
docker run --rm Panonim/dynacat password:hash mysecretpassword
```

Then use `password-hash` instead of `password`:

```yaml
auth:
  secret-key: ...
  users:
    admin:
      password-hash: $2a$10$o6SXqiccI3DDP2dN4ADumuOeIHET6Q4bUMYZD6rT2Aqt6XQ3DyO.6
```

### Brute-force protection

Dynacat automatically blocks IPs that fail to authenticate 5 times within 5 minutes. For this to work correctly behind a reverse proxy, set:

```yaml
server:
  proxied: true
```

This tells Dynacat to read the real client IP from the `X-Forwarded-For` header.

---

## OIDC Authentication

Integrate with any OpenID Connect identity provider (Authentik, Authelia, PocketID, etc.) using the OAuth2 Authorization Code flow with PKCE.

### Basic configuration

```yaml
auth:
  secret-key: # generate with: ./dynacat secret:make
  oidc:
    issuer-url: https://auth.example.com
    client-id: dynacat
    client-secret: ${secret:oidc_client_secret}
    redirect-url: https://dashboard.example.com/api/oidc/callback
```

### Full OIDC options

```yaml
auth:
  secret-key: ...
  oidc:
    issuer-url: https://auth.example.com       # required: OpenID Connect issuer URL
    client-id: dynacat                          # required
    client-secret: ${OIDC_CLIENT_SECRET}        # required
    redirect-url: https://dashboard.example.com/api/oidc/callback  # required
    scopes:                                     # optional, defaults shown
      - openid
      - profile
      - email
    username-claim: preferred_username          # optional, default: preferred_username
    groups-claim: groups                        # optional, default: groups
    allowed-groups:                             # optional: restrict OIDC login to these IdP groups
      - homelab-users
    allowed-users:                              # optional: restrict OIDC login to these usernames
      - alice
```

The `redirect-url` must match the callback URL configured in your identity provider. The path must end with `/api/oidc/callback`.

### Disabling password auth (OIDC-only mode)

When OIDC is working and you want to remove the password login option entirely:

```yaml
auth:
  secret-key: ...
  disable-password: true
  oidc:
    issuer-url: https://auth.example.com
    client-id: dynacat
    client-secret: ${OIDC_CLIENT_SECRET}
    redirect-url: https://dashboard.example.com/api/oidc/callback
```

### Provider guides

#### Generic OIDC

Register a new OAuth2/OIDC application in your identity provider:
- **Redirect URI**: `https://dashboard.example.com/api/oidc/callback`
- **Grant type**: Authorization Code
- **Scopes**: `openid profile email` (add `groups` if your provider supports it)

Then configure Dynacat with the issuer URL, client ID, and client secret from your provider.

#### Authentik

1. In Authentik, go to **Applications → Providers → Create → OAuth2/OpenID Provider**
2. Set **Redirect URIs** to `https://dashboard.example.com/api/oidc/callback`
3. Set **Scopes** to include `openid`, `profile`, `email`, and optionally `groups`
4. Copy the **Client ID** and **Client Secret**
5. Use the provider's OIDC issuer URL, not the bare Authentik instance URL. In Authentik this usually looks like `https://auth.example.com/application/o/<provider-slug>/` and is the URL Dynacat should use for discovery.

```yaml
auth:
  oidc:
    issuer-url: https://auth.example.com/application/o/<provider-slug>/
    client-id: <client-id-from-authentik>
    client-secret: ${OIDC_CLIENT_SECRET}
    redirect-url: https://dashboard.example.com/api/oidc/callback
    scopes:
      - openid
      - profile
      - email
      - groups
    groups-claim: groups
```

#### Authelia

1. In Authelia's configuration, add a new client under `identity_providers.oidc.clients`:

```yaml
identity_providers:
  oidc:
    clients:
      - client_id: dynacat
        client_secret: '$pbkdf2-sha512$...'  # hashed secret
        redirect_uris:
          - https://dashboard.example.com/api/oidc/callback
        scopes:
          - openid
          - profile
          - email
          - groups
        grant_types:
          - authorization_code
```

2. In Dynacat:

```yaml
auth:
  oidc:
    issuer-url: https://auth.example.com
    client-id: dynacat
    client-secret: ${OIDC_CLIENT_SECRET}
    redirect-url: https://dashboard.example.com/api/oidc/callback
    scopes:
      - openid
      - profile
      - email
      - groups
```

#### PocketID

1. In PocketID, create a new OIDC client
2. Set the **Callback URL** to `https://dashboard.example.com/api/oidc/callback`
3. Copy the client ID and secret

```yaml
auth:
  oidc:
    issuer-url: https://pocketid.example.com
    client-id: <client-id>
    client-secret: ${OIDC_CLIENT_SECRET}
    redirect-url: https://dashboard.example.com/api/oidc/callback
```

---

## Per-Page Access Control

Restrict individual pages to specific users or groups using `allowed-users` and `allowed-groups`:

```yaml
pages:
  - name: Public Status
    # no allowed-users/groups = accessible to all authenticated users
    columns: ...

  - name: Admin Dashboard
    allowed-users:
      - admin
    allowed-groups:
      - devops
    columns: ...
```

A user can access a restricted page if their username appears in `allowed-users` **or** they belong to any group in `allowed-groups`.

Pages without `allowed-users` or `allowed-groups` are accessible to all authenticated users.

### Guest / public access

Set `require-auth: false` to allow unauthenticated access to unrestricted pages:

```yaml
auth:
  require-auth: false
  secret-key: ...
  users:
    admin:
      password-hash: ...
```

With `require-auth: false`:
- Pages with no `allowed-users`/`allowed-groups` are publicly accessible
- Pages with `allowed-users`/`allowed-groups` still require login + correct access
- A login button appears in the navigation bar for unauthenticated visitors

### Combined example: public status + private admin

```yaml
auth:
  secret-key: ${secret:auth_secret}
  require-auth: false
  oidc:
    issuer-url: https://auth.example.com
    client-id: dynacat
    client-secret: ${secret:oidc_secret}
    redirect-url: https://dashboard.example.com/api/oidc/callback
    groups-claim: groups

pages:
  - name: Status
    # public - no restrictions
    columns:
      - size: full
        widgets:
          - type: monitor
            ...

  - name: Admin
    allowed-groups:
      - admins
    columns:
      - size: full
        widgets:
          - type: server-stats
            ...
```

---

## Docker / Environment Variables

All OIDC fields support the existing `${ENV_VAR}` and `${secret:name}` syntax:

```yaml
auth:
  secret-key: ${secret:auth_secret}
  oidc:
    issuer-url: ${OIDC_ISSUER_URL}
    client-id: ${OIDC_CLIENT_ID}
    client-secret: ${secret:oidc_client_secret}
    redirect-url: ${OIDC_REDIRECT_URL}
```

Docker Compose example:

```yaml
services:
  dynacat:
    image: panonim/dynacat
    volumes:
      - ./config:/app/config
    environment:
      - OIDC_ISSUER_URL=https://auth.example.com
      - OIDC_CLIENT_ID=dynacat
      - OIDC_REDIRECT_URL=https://dashboard.example.com/api/oidc/callback
    secrets:
      - auth_secret
      - oidc_client_secret

secrets:
  auth_secret:
    file: ./auth_secret.txt
  oidc_client_secret:
    file: ./oidc_secret.txt
```
