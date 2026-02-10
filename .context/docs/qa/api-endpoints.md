---
status: filled
slug: api-endpoints
category: features
---

# What API endpoints are available?

## API Architecture

The Automatiza platform uses **ConnectRPC** (gRPC-Web compatible) for all API communication.

- **Base URL:** `/connect`
- **Protocol:** HTTP/2 with Protocol Buffers
- **Content-Type:** `application/grpc-web+proto`

## Service Catalog (26 Services)

### Core Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **AuthService** | `auth.v1` | Login, Register, Logout, RefreshSession, PasswordReset, MagicLink |
| **ProfilesService** | `profiles.v1` | GetProfile, UpdateProfile, UpdatePassword |
| **DashboardService** | `dashboard.v1` | GetDashboardStats, GetAnnouncement |

### WhatsApp Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **WhatsAppService** | `whatsapp.v1` | CreateInstance, DeleteInstance, Reconnect, CheckStatus, FetchGroups |
| **InstancesService** | `instances.v1` | ListInstances, GetLatestInstance, UpsertStatus |
| **GroupsService** | `groups.v1` | List, Get, Create, Update, Delete, Sync, SendMessage, SendAudio, SendImage, SendPoll |

### Automation Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **AutomationsService** | `automations.v1` | CRUD, Run, Diagnose (120s timeout), ResetHistory, ListDispatchLogs |
| **JobsService** | `jobs.v1` | CRUD, GetQueueMetrics |
| **DispatchTextsService** | `dispatch_texts.v1` | CRUD for message templates |

### Commerce Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **CouponsService** | `coupons.v1` | List, Get, Create, Update, Delete |
| **MlLinksService** | `ml_links.v1` | CRUD for Mercado Livre links |
| **RedirectService** | `redirect.v1` | CreateLink, GetStats, ListLinks |
| **CampaignsService** | `campaigns.v1` | GenerateCampaign (AI-powered) |

### Affiliate Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **AffiliatesService** | `affiliates.v1` | Credentials, Shopee, MercadoLivre integration |

### Admin Services

| Service | Package | Key Operations |
|---------|---------|----------------|
| **AdminService** | `admin.v1` | GetObservability, GetQueueMetrics, GetDispatchAnalytics |
| **LoadBalancerService** | `admin.v1` | Server capacity management |

### Other Services

| Service | Package |
|---------|---------|
| **HistoryService** | `history.v1` |
| **SettingsService** | `settings.v1` |
| **WebhooksService** | `webhooks.v1` |
| **ImagesService** | `images.v1` |
| **PlansService** | `plans.v1` |
| **AIService** | `ai.v1` |

## Request/Response Examples

### List Groups
```typescript
// Request
const client = createClient(GroupsService, transport);
const response = await client.listGroups({
  instanceId: 'inst-123',
  pagination: { limit: 50, offset: 0 }
});

// Response
{
  groups: [
    { id: 'grp-1', name: 'Marketing', memberCount: 150, ... },
    ...
  ],
  totalCount: 120
}
```

### Create Automation
```typescript
const response = await client.createAutomation({
  name: 'Offer Sender',
  mode: AutomationMode.SEARCH,
  schedule: { startTime: '08:00', endTime: '22:00' },
  config: { stores: ['shopee'], keywords: ['promoção'] }
});
```

## Error Handling

All errors follow gRPC status codes:

| Code | Meaning | Frontend Action |
|------|---------|-----------------|
| `InvalidArgument` | Validation error | Show inline error |
| `Unauthenticated` | Not logged in | Redirect to /auth |
| `PermissionDenied` | Access denied | Show toast |
| `NotFound` | Resource missing | Show toast |
| `ResourceExhausted` | Rate limited | Retry with backoff |
| `Unavailable` | Service down | Retry with backoff |

## Transport Configuration

```typescript
// client/src/lib/connect-transport.ts
const transport = createConnectTransport({
  baseUrl: '/connect',
  interceptors: [
    csrfInterceptor,      // Adds X-CSRF-Token
    tracingInterceptor,   // Adds traceparent header
    retryInterceptor,     // Exponential backoff
    errorNormalizer,      // Error logging
  ],
});
```

## Proto Files Location

All service definitions in `proto/*/v1/*.proto`:
- Generated Go code: `automatiza-go/internal/gen/`
- Generated TypeScript: `client/src/gen/`
