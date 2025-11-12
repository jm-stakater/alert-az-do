# alert-az-do

[![Build Status](https://github.com/jm-stakater/alert-az-do/actions/workflows/test.yaml/badge.svg?branch=master)](https://github.com/jm-stakater/alert-az-do/actions?query=workflow%3Atest)
[![Go Report Card](https://goreportcard.com/badge/github.com/jm-stakater/alert-az-do)](https://goreportcard.com/report/github.com/jm-stakater/alert-az-do) 
[![GoDoc](https://godoc.org/github.com/jm-stakater/alert-az-do?status.svg)](https://godoc.org/github.com/jm-stakater/alert-az-do)
[![Slack](https://img.shields.io/badge/join%20slack-%23alert-az-do-brightgreen.svg)](https://slack.cncf.io/)

[Prometheus Alertmanager](https://github.com/prometheus/alertmanager) webhook receiver for [Azure DevOps](https://azure.microsoft.com/en-us/products/devops/).

## Overview

alert-az-do implements Alertmanager's webhook HTTP API and connects to one or more Azure DevOps organizations to create highly configurable Azure DevOps work items. One work item is created per distinct group key — as defined by the [`group_by`](https://prometheus.io/docs/alerting/configuration/#<route>) parameter of Alertmanager's `route` configuration section — but not closed when the alert is resolved. The expectation is that a human will look at the work item, take any necessary action, then close it. If no human interaction is necessary then it should probably not alert in the first place. This behavior however can be modified by setting the `auto_resolve` section, which will resolve the Azure DevOps work item with the required state.

If a corresponding Azure DevOps work item already exists but is resolved, it is reopened. An Azure DevOps state transition must exist between the resolved state and the reopened state — as defined by `reopen_state` — or reopening will fail. Optionally a "skip reopen state" — defined by `skip_reopen_state` — may be defined: an Azure DevOps work item in this state will not be reopened by alert-az-do (e.g., work items marked as "Removed" or "Cut").

## Features

- **Multiple Authentication Methods**: Support for both Azure AD OAuth (Client ID/Secret) and Personal Access Tokens (PAT)
- **Flexible Work Item Creation**: Create different types of work items (Bug, Task, Issue, etc.) based on alert content
- **Template-based Content**: Use Go templates to generate dynamic work item titles, descriptions, and field values
- **Auto-resolution**: Automatically resolve work items when alerts are resolved
- **Custom Fields**: Set standard and custom Azure DevOps fields using templates
- **Multi-project Support**: Search across multiple projects for existing work items
- **Label Management**: Copy Prometheus labels as Azure DevOps tags
- **Update Modes**: Choose between updating work items directly or adding comments

## Usage

Get alert-az-do, either as a [packaged release](https://github.com/jm-stakater/alert-az-do/releases) or build it yourself:

```bash
go get github.com/jm-stakater/alert-az-do/cmd/alert-az-do
```

then run it from the command line:

```bash
alert-az-do
```

Use the `-help` flag to get help information.

```bash
alert-az-do -help
Usage of alert-az-do:
  -config string
      The alert-az-do configuration file (default "config/alert-az-do.yml")
  -listen-address string
      The address to listen on for HTTP requests. (default ":9097")
  -log-level string
      Only log messages with the given severity or above (debug, info, warn, error) (default "info")
  -log-format string
      Output format of log messages (logfmt, json) (default "logfmt")
```

## Testing

alert-az-do expects a JSON object from Alertmanager. The format of this JSON is described in the [Alertmanager documentation](https://prometheus.io/docs/alerting/configuration/#<webhook_config>) or, alternatively, in the [Alertmanager GoDoc](https://godoc.org/github.com/prometheus/alertmanager/template#Data).

To quickly test if alert-az-do is working you can run:

```bash
curl -H "Content-type: application/json" -X POST \
  -d '{"receiver": "contoso-ab", "status": "firing", "alerts": [{"status": "firing", "labels": {"alertname": "TestAlert", "severity": "critical"} }], "groupLabels": {"alertname": "TestAlert"}}' \
  http://localhost:9097/alert
```

## Configuration

The configuration file is essentially a list of receivers matching 1-to-1 all Alertmanager receivers using alert-az-do; plus defaults (in the form of a partially defined receiver); and a pointer to the template file.

Each receiver must have a unique name (matching the Alertmanager receiver name), Azure DevOps API access fields (organization, authentication credentials), a handful of required work item fields (such as the Azure DevOps project and work item summary), some optional work item fields (e.g. priority, area path, iteration path) and a `fields` map for other (standard or custom) Azure DevOps fields. Most of these may use [Go templating](https://golang.org/pkg/text/template/) to generate the actual field values based on the contents of the Alertmanager notification. The exact same data structures and functions as those defined in the [Alertmanager template reference](https://prometheus.io/docs/alerting/notifications/) are available in alert-az-do.

Similar to Alertmanager, alert-az-do supports environment variable substitution with the `$(...)` syntax.

### Authentication

alert-az-do supports two authentication methods:

1. **Azure AD OAuth** (recommended for production):
   ```yaml
   organization: contoso
   tenant_id: your-tenant-id
   client_id: your-client-id
   client_secret: $(CLIENT_SECRET)
   ```

2. **Personal Access Token**:
   ```yaml
   organization: contoso
   personal_access_token: $(PAT_TOKEN)
   ```

### Example Configuration

```yaml
# Global defaults
defaults:
  organization: contoso
  tenant_id: $(TENANT_ID)
  client_id: $(CLIENT_ID)
  client_secret: $(CLIENT_SECRET)
  
  issue_type: Bug
  priority: '{{ template "azdo.priority" . }}'
  summary: '{{ template "azdo.summary" . }}'
  description: '{{ template "azdo.description" . }}'
  reopen_state: "To Do"
  skip_reopen_state: "Removed"
  
receivers:
  - name: 'team-alpha'
    project: TeamAlpha
    add_group_labels: true
    
  - name: 'team-beta'
    project: TeamBeta
    issue_type: Task
    fields:
      System.AssignedTo: '{{ .CommonLabels.owner }}'
      Custom.Priority: '{{ .CommonLabels.severity }}'

template: alert-az-do.tmpl
```

### Template Functions

alert-az-do provides additional template functions beyond the standard Alertmanager functions:

- `toUpper`: Convert string to uppercase
- `toLower`: Convert string to lowercase  
- `join`: Join string slice with separator
- `match`: Test if string matches regex pattern
- `reReplaceAll`: Replace all regex matches in string
- `stringSlice`: Create a string slice from arguments
- `getEnv`: Get environment variable value

## Alertmanager Configuration

To enable Alertmanager to talk to alert-az-do you need to configure a webhook in Alertmanager. You can do that by adding a webhook receiver to your Alertmanager configuration.

```yaml
receivers:
- name: 'team-alpha'
  webhook_configs:
  - url: 'http://localhost:9097/alert'
    # Send resolved alerts if you want auto-resolution
    send_resolved: true
```

## Azure DevOps Setup

### Permissions Required

The Azure AD application or Personal Access Token needs the following permissions:

- **Work Items**: Read & write
- **Project and team**: Read (to access project information)

### Creating a Personal Access Token

1. Go to Azure DevOps → User Settings → Personal Access Tokens
2. Create a new token with "Work Items (Read & write)" scope
3. Use the token in your configuration

### Creating an Azure AD Application

1. Go to Azure Portal → Azure Active Directory → App registrations
2. Create a new application
3. Note the Application (client) ID and Directory (tenant) ID
4. Create a client secret in "Certificates & secrets"
5. Add the application to your Azure DevOps organization with appropriate permissions

## Profiling

alert-az-do imports [`net/http/pprof`](https://golang.org/pkg/net/http/pprof/) to expose runtime profiling data on the `/debug/pprof` endpoint. For example, to use the pprof tool to look at a 30-second CPU profile:

```bash
go tool pprof http://localhost:9097/debug/pprof/profile
```

To enable mutex and block profiling (i.e. `/debug/pprof/mutex` and `/debug/pprof/block`) run alert-az-do with the `DEBUG` environment variable set:

```bash
env DEBUG=1 ./alert-az-do
```

## Docker

alert-az-do is available as a Docker image:

```bash
docker run -v $(pwd)/config:/config \
  -p 9097:9097 \
  ghcr.io/jm-stakater/alert-az-do:latest \
  -config /config/alert-az-do.yml
```

## Contributing

We welcome contributions! Please see our [contributing guidelines](CONTRIBUTING.md) for details.

## Community

alert-az-do is an open source project and we welcome new contributors and members of the community. Here are ways to get in touch with the community:

* Issue Tracker: [GitHub Issues](https://github.com/jm-stakater/alert-az-do/issues)

## License

alert-az-do is licensed under the [Apache License 2.0](https://github.com/jm-stakater/alert-az-do/blob/master/LICENSE).

Copyright (c) 2025 Stakater AB
