# CLI: send API key requests to the BFF REST API

Ticket: RSC-1377.

## Why

The BFF accepts organization API keys only on `/api/v1` and MCP; `/graphql` refuses them
(resim-ai/rerun#4452 and #4453). CLI v1.2.0 sends its metrics config and dashboard operations
to GraphQL with whatever credential it has, so with a key those commands fail once the BFF
change ships.

## What

- `bff.NewRESTClient` implements genqlient's `graphql.Client` for the nine generated
  operations, mapping each to its `/api/v1` route and the REST response back to the
  generated response type:

  | Operation | Route |
  |---|---|
  | `GetConfigFileSchema` | `GET /metrics-config/schema` |
  | `ValidateMetricsConfig` | `POST /metrics-config/validate` |
  | `ValidateMetricsSet` | `POST /metrics-config/validate-metrics-set` |
  | `FindUnusedMetrics` | `POST /metrics-config/unused-metrics` |
  | `PreviewTopicRemoval` | `POST /metrics-config/topic-removal-preview` |
  | `UpdateMetricsConfig` | `PUT /metrics-config` |
  | `CreateDashboard` | `POST /dashboards` |
  | `GetDashboard` | `GET /dashboards/{id}` |
  | `CreateDebugDashboard` | `POST /debug-dashboards` |

  The generated variables already use the REST field names. REST errors come back as a
  `gqlerror.List` (one entry per validation error, with the error code in `extensions`), so
  the commands' error handling is unchanged.
- `auth.NewBFFClient` returns that client when the CLI is signed in with an API key, at the
  BFF host with path `/api/v1`. Other sign-ins keep the GraphQL client. No command changes.
- `TestMetricsWithAPIKey` (end to end) syncs, validates and creates a dashboard with a read &
  write key from `SIGNALFLAG_E2E_API_KEY`, and is skipped when it is not set. CI passes the
  repo secret of the same name.
- CHANGELOG: v1.3.0.

## Release

Release v1.3.0 before the BFF change reaches production. The e2e secret should be added once
the BFF change is on staging, since the CLI e2e runs against staging.
