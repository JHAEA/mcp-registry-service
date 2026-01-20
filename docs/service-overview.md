# MCP Registry Service Overview

## Executive Summary

The MCP Registry is an internal service that provides a centralized catalog of AI tool integrations (called "MCP servers") that our AI assistants can discover and use. It enables secure, version-controlled management of these integrations through a GitOps workflow—meaning all changes go through pull requests with proper review and approval.

---

## What Problem Does This Solve?

As we adopt AI assistants across the organization, we need a way to:

1. **Discover available integrations** — What tools can our AI assistants connect to?
2. **Standardize configuration** — How should each integration be configured?
3. **Control access** — Who can add or modify integrations?
4. **Track changes** — What changed, when, and by whom?

Without a registry, teams would manage integrations ad-hoc, leading to inconsistent configurations, security gaps, and no visibility into what's available.

---

## How It Works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         GitOps Workflow                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Developer              GitHub Repository           Registry API    │
│       │                        │                          │          │
│       │  1. Submit PR          │                          │          │
│       │ ──────────────────────>│                          │          │
│       │                        │                          │          │
│       │  2. Auto-validate      │                          │          │
│       │    (CI checks)         │                          │          │
│       │ <──────────────────────│                          │          │
│       │                        │                          │          │
│       │  3. Review & Approve   │                          │          │
│       │ ──────────────────────>│                          │          │
│       │                        │                          │          │
│       │                        │  4. Webhook notification │          │
│       │                        │ ────────────────────────>│          │
│       │                        │                          │          │
│       │                        │  5. Pull & update cache  │          │
│       │                        │ <────────────────────────│          │
│       │                        │                          │          │
│   AI Assistant                 │  6. Query available      │          │
│       │                        │     servers              │          │
│       │ ─────────────────────────────────────────────────>│          │
│       │                        │                          │          │
│       │                        │  7. Return server list   │          │
│       │ <─────────────────────────────────────────────────│          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### The Two Components

| Component | Purpose | Location |
|-----------|---------|----------|
| **Data Repository** | Stores server definitions as YAML files | GitHub: `JHAEA/askjack-mcp-registry` |
| **API Server** | Serves the catalog to AI assistants | Runs in Docker container |

---

## Key Features

### 1. GitOps-Based Change Management
All changes to the registry go through GitHub pull requests:
- **Automatic validation** — CI checks ensure configurations are valid before merge
- **Review required** — Changes need approval before going live
- **Full audit trail** — Git history shows who changed what and when
- **Easy rollback** — Revert any change with a single git command

### 2. Read-Only API
The API only serves data—it cannot modify the registry. This ensures:
- All changes flow through the approved PR process
- No backdoor modifications
- Clear separation of concerns

### 3. Automatic Synchronization
When changes merge to main:
- GitHub sends a webhook notification
- The API server pulls the latest data within seconds
- No manual intervention required

### 4. Production-Ready Security
- **GitHub App authentication** — Secure access to private repositories
- **Webhook signature verification** — Prevents spoofed notifications
- **Hardened container** — Minimal attack surface, non-root user, read-only filesystem
- **No secrets in code** — All credentials via environment variables

---

## What's in the Registry?

Each entry in the registry describes an MCP server with:

| Field | Description | Example |
|-------|-------------|---------|
| **Name** | Unique identifier | `io.github.jhaea/askjack-demo` |
| **Description** | What it does | "AskJack demo MCP server" |
| **Version** | Semantic version | `1.1.0` |
| **Transport** | How to connect | HTTP endpoint or local process |
| **Configuration** | Required settings | API keys, headers, etc. |

### Example Server Definition

```yaml
name: io.github.jhaea/askjack-demo
description: AskJack demo MCP server
version: "1.1.0"

remotes:
  - type: streamable-http
    url: https://api.askjack.example.com/mcp
    headers:
      - name: Authorization
        description: Bearer token for API authentication
        isRequired: true
        isSecret: true
```

---

## API Endpoints

The service exposes a simple REST API:

| Endpoint | Purpose |
|----------|---------|
| `GET /v0.1/servers` | List all available servers |
| `GET /v0.1/servers/{name}` | Get details for a specific server |
| `GET /v0.1/health` | Service health and sync status |
| `GET /metrics` | Prometheus metrics for monitoring |

### Example: List Servers

```bash
curl https://registry.internal.example.com/v0.1/servers
```

```json
{
  "servers": [
    {
      "server": {
        "name": "io.github.jhaea/askjack-demo",
        "description": "AskJack demo MCP server",
        "version": "1.1.0"
      }
    }
  ],
  "metadata": {
    "count": 1
  }
}
```

---

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                        Infrastructure                           │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐      ┌─────────────────────────────────────┐  │
│  │   GitHub    │      │         Docker Container            │  │
│  │             │      │  ┌─────────────────────────────┐    │  │
│  │  ┌───────┐  │      │  │     MCP Registry Server     │    │  │
│  │  │ YAML  │  │ Pull │  │                             │    │  │
│  │  │ Files │──┼──────┼──│  • Go application           │    │  │
│  │  └───────┘  │      │  │  • In-memory cache (LRU)    │    │  │
│  │             │      │  │  • Prometheus metrics       │    │  │
│  │  ┌───────┐  │      │  │  • OpenTelemetry tracing    │    │  │
│  │  │  CI   │  │      │  │                             │    │  │
│  │  │Checks │  │      │  └──────────────┬──────────────┘    │  │
│  │  └───────┘  │      │                 │ :8080             │  │
│  │             │      └─────────────────┼───────────────────┘  │
│  └─────────────┘                        │                      │
│                                         ▼                      │
│                              ┌─────────────────┐               │
│                              │  Load Balancer  │               │
│                              └────────┬────────┘               │
│                                       │                        │
│                                       ▼                        │
│                              AI Assistants / Clients           │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go 1.22 | Fast, secure, single binary deployment |
| Container | Distroless | Minimal attack surface |
| Data Store | Git repository | Version control, audit trail, familiar workflow |
| Cache | In-memory LRU | Fast reads, automatic eviction |
| Monitoring | Prometheus + OpenTelemetry | Industry standard observability |

---

## Security Considerations

| Concern | Mitigation |
|---------|------------|
| Unauthorized changes | All changes require PR approval |
| API tampering | API is read-only; data comes from Git |
| Container escape | Distroless base, non-root user, dropped capabilities |
| Credential exposure | Secrets in environment variables, not code |
| Webhook spoofing | HMAC-SHA256 signature verification |
| Supply chain | Dependency scanning in CI, minimal dependencies |

---

## Operational Benefits

### For Developers
- Add new integrations via familiar PR workflow
- Automatic validation catches errors before merge
- Self-service—no tickets needed for standard changes

### For Operations
- Single source of truth for all AI integrations
- Easy to audit and comply with policies
- Standard monitoring and alerting

### For Security
- All changes are reviewed and approved
- Complete audit trail in Git
- No direct write access to production data

---

## Current Status

| Item | Status |
|------|--------|
| API Server | ✅ Implemented and tested |
| GitHub App Authentication | ✅ Configured |
| Webhook Sync | ✅ Working |
| CI Validation | ✅ Deployed |
| Index Auto-generation | ✅ Working |
| Sample Server | ✅ Added |

### Repository Links

- **API Server Code**: `mcp-registry` (this repository)
- **Registry Data**: [github.com/JHAEA/askjack-mcp-registry](https://github.com/JHAEA/askjack-mcp-registry)

---

## Next Steps

1. **Deploy to staging** — Run the containerized service in a staging environment
2. **Configure monitoring** — Connect Prometheus metrics to alerting
3. **Onboard first integration** — Add a real MCP server to the registry
4. **Document onboarding process** — Create guide for teams adding integrations
5. **Production deployment** — Deploy with proper TLS and load balancing

---

## Questions?

Contact the platform team for:
- Adding new servers to the registry
- API access and integration
- Infrastructure and deployment questions
