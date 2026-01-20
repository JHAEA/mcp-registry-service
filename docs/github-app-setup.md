# GitHub App Setup Guide

This guide walks you through creating and configuring a GitHub App for the MCP Registry server.

## Create a GitHub App

1. Go to your GitHub organization or personal settings
2. Navigate to **Developer settings** → **GitHub Apps** → **New GitHub App**

### App Configuration

| Field | Value |
|-------|-------|
| **GitHub App name** | `MCP Registry - <your-org>` |
| **Homepage URL** | Your registry URL or repository |
| **Webhook URL** | `https://askjack.jackhenry.com/webhooks/mcp-registry` |
| **Webhook secret** | Generate a secure random string (f9R*xoLkwej0M<3jushPwF>r) |

### Permissions

Under **Repository permissions**, set:

| Permission | Access |
|------------|--------|
| **Contents** | Read-only |
| **Metadata** | Read-only (automatically selected) |

All other permissions should remain "No access".

### Subscribe to Events

Under **Subscribe to events**, check:

- [x] **Push** — Notifies when commits are pushed

### Where can this GitHub App be installed?

Choose based on your needs:
- **Only on this account** — For private/internal registries
- **Any account** — For public registry apps

## After Creation

### 1. Generate Private Key

1. Scroll down to **Private keys**
2. Click **Generate a private key**
3. Save the downloaded `.pem` file securely

### 2. Note Your App ID
2664202
The **App ID** is displayed at the top of the App settings page.

### 3. Install the App

1. Click **Install App** in the left sidebar
2. Select the organization or account
3. Choose **Only select repositories**
4. Select your registry data repository
5. Click **Install**

### 4. Get Installation ID

After installation, you'll be redirected to a URL like:
```
https://github.com/settings/installations/12345678
```

The number (`104401507`) is your **Installation ID**.

## Configure the Registry Server

Set these environment variables:

```bash
# GitHub App ID (from App settings page)
GITHUB_APP_ID=123456

# Installation ID (from URL after installing)
GITHUB_INSTALLATION_ID=12345678

# Private key (either path or content)
GITHUB_APP_PRIVATE_KEY_PATH=/path/to/your-app.private-key.pem
# OR
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----"

# Webhook secret (the one you generated during App creation)
WEBHOOK_SECRET=your-webhook-secret
```

## Webhook Configuration

### Verify Webhook Delivery

1. Go to your GitHub App settings
2. Click **Advanced** in the left sidebar
3. Check **Recent Deliveries** after pushing to the repository

### Webhook Payload URL

The webhook should point to your registry's webhook endpoint:
```
https://your-registry.example.com/webhooks/github
```

### Content Type

Ensure the webhook is configured to send:
- **Content type**: `application/json`

### SSL Verification

For production, always enable **SSL verification**.

## Troubleshooting

### "Bad credentials" Error

- Verify the App ID is correct
- Check that the private key matches the App
- Ensure the Installation ID is for the correct repository

### "Resource not accessible by integration" Error

- Verify the App is installed on the repository
- Check that **Contents** permission is set to **Read**

### Webhook Not Triggering Sync

1. Check the **Recent Deliveries** in GitHub App settings
2. Verify the webhook secret matches
3. Ensure the webhook URL is accessible from GitHub
4. Check server logs for signature verification errors

### Token Refresh Issues

GitHub App installation tokens expire after 1 hour. The server automatically refreshes tokens using the `ghinstallation` library. If you see authentication errors:

1. Verify the private key is valid and not corrupted
2. Check system clock is synchronized (tokens are time-sensitive)
3. Regenerate the private key if necessary

## Security Best Practices

1. **Private Key Storage**
   - Never commit the private key to version control
   - Use a secrets manager (Vault, AWS Secrets Manager, etc.)
   - Rotate keys periodically

2. **Webhook Secret**
   - Use a cryptographically random string (32+ characters)
   - Rotate the secret if compromised

3. **Minimal Permissions**
   - Only grant **Contents: Read** permission
   - Don't enable unnecessary event subscriptions

4. **Installation Scope**
   - Only install on repositories that need it
   - Use "Only select repositories" option
