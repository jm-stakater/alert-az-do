# alert-az-do
[![Build Status](https://github.com/jm-stakater/alert-az-do/workflows/test/badge.svg?branch=master)](https://github.com/jm-stakater/alert-az-do/actions?query=workflow%3Atest) 
[![Go Report Card](https://goreportcard.com/badge/github.com/jm-stakater/alert-az-do)](https://goreportcard.com/report/github.com/jm-stakater/alert-az-do) 
[![GoDoc](https://godoc.org/github.com/jm-stakater/alert-az-do?status.svg)](https://godoc.org/github.com/jm-stakater/alert-az-do)
[![Slack](https://img.shields.io/badge/join%20slack-%23alert-az-do-brightgreen.svg)](https://slack.cncf.io/)
[Prometheus Alertmanager](https://github.com/prometheus/alertmanager) webhook receiver for [Azure DevOps](https://azure.microsoft.com/en-us/products/devops/).

## Overview

Azure-DevOps implements Alertmanager's webhook HTTP API and connects to one or more JIRA instances to create highly configurable JIRA issues. One issue is created per distinct group key — as defined by the [`group_by`](https://prometheus.io/docs/alerting/configuration/#<route>) parameter of Alertmanager's `route` configuration section — but not closed when the alert is resolved. The expectation is that a human will look at the issue, take any necessary action, then close it.  If no human interaction is necessary then it should probably not alert in the first place. This behavior however can be modified by setting `auto_resolve` section, which will resolve the jira issue with required state.

If a corresponding JIRA issue already exists but is resolved, it is reopened. A JIRA transition must exist between the resolved state and the reopened state — as defined by `reopen_state` — or reopening will fail. Optionally a "won't fix" resolution — defined by `wont_fix_resolution` — may be defined: a JIRA issue with this resolution will not be reopened by alert-az-do.

## Usage

Get alert-az-do, either as a [packaged release](https://github.com/jm-stakater/alert-az-do/releases) or build it yourself:

```
$ go get github.com/jm-stakater/alert-az-do/cmd/alert-az-do
```

then run it from the command line:

```
$ alert-az-do
```

Use the `-help` flag to get help information.

```
$ alert-az-do -help
Usage of alert-az-do:
  -config string
      The alert-az-do configuration file (default "config/alert-az-do.yml")
  -listen-address string
      The address to listen on for HTTP requests. (default ":9097")
  [...]
```

## Testing

alert-az-do expects a JSON object from Alertmanager. The format of this JSON is described in the [Alertmanager documentation](https://prometheus.io/docs/alerting/configuration/#<webhook_config>) or, alternatively, in the [Alertmanager GoDoc](https://godoc.org/github.com/prometheus/alertmanager/template#Data).

To quickly test if alert-az-do is working you can run:

```bash
$ curl -H "Content-type: application/json" -X POST \
  -d '{"receiver": "jira-ab", "status": "firing", "alerts": [{"status": "firing", "labels": {"alertname": "TestAlert", "key": "value"} }], "groupLabels": {"alertname": "TestAlert"}}' \
  http://localhost:9097/alert
```

## Configuration

The configuration file is essentially a list of receivers matching 1-to-1 all Alertmanager receivers using alert-az-do; plus defaults (in the form of a partially defined receiver); and a pointer to the template file.

Each receiver must have a unique name (matching the Alertmanager receiver name), JIRA API access fields (URL, username and password), a handful of required issue fields (such as the JIRA project and issue summary), some optional issue fields (e.g. priority) and a `fields` map for other (standard or custom) JIRA fields. Most of these may use [Go templating](https://golang.org/pkg/text/template/) to generate the actual field values based on the contents of the Alertmanager notification. The exact same data structures and functions as those defined in the [Alertmanager template reference](https://prometheus.io/docs/alerting/notifications/) are available in alert-az-do.

Similar to Alertmanager, alert-az-do supports environment variable substitution with the `$(...)` syntax.

## Alertmanager configuration

To enable Alertmanager to talk to alert-az-do you need to configure a webhook in Alertmanager. You can do that by adding a webhook receiver to your Alertmanager configuration. 

```yaml
receivers:
- name: 'jira-ab'
  webhook_configs:
  - url: 'http://localhost:9097/alert'
    # alert-az-do ignores resolved alerts, avoid unnecessary noise
    send_resolved: false
```

## Profiling

alert-az-do imports [`net/http/pprof`](https://golang.org/pkg/net/http/pprof/) to expose runtime profiling data on the `/debug/pprof` endpoint. For example, to use the pprof tool to look at a 30-second CPU profile:

```bash
go tool pprof http://localhost:9097/debug/pprof/profile
```

To enable mutex and block profiling (i.e. `/debug/pprof/mutex` and `/debug/pprof/block`) run alert-az-do with the `DEBUG` environment variable set:

```bash
env DEBUG=1 ./alert-az-do
```

## Community

*Azure-DevOps* is an open source project and we welcome new contributors and members 
of the community. Here are ways to get in touch with the community:

* Issue Tracker: [GitHub Issues](https://github.com/jm-stakater/alert-az-do/issues)

## License

alert-az-do is licensed under the [MIT License](https://github.com/jm-stakater/alert-az-do/blob/master/LICENSE).

Copyright (c) 2025, Jonas Marklén
