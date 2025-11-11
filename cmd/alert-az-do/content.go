// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/jm-stakater/alert-az-do/pkg/alertmanager"
	"github.com/jm-stakater/alert-az-do/pkg/config"
	"github.com/jm-stakater/alert-az-do/pkg/notify"
	tmpl "github.com/jm-stakater/alert-az-do/pkg/template"
	v7 "github.com/microsoft/azure-devops-go-api/azuredevops/v7"

	_ "net/http/pprof"
)

const (
	docsURL   = "https://github.com/jm-stakater/alert-az-do#readme"
	templates = `
    {{ define "page" -}}
      <html>
      <head>
        <title>alert-az-do</title>
        <style type="text/css">
          body { margin: 0; font-family: "Helvetica Neue", Helvetica, Arial, sans-serif; font-size: 14px; line-height: 1.42857143; color: #333; background-color: #fff; }
          .navbar { display: flex; background-color: #222; margin: 0; border-width: 0 0 1px; border-style: solid; border-color: #080808; }
          .navbar > * { margin: 0; padding: 15px; }
          .navbar * { line-height: 20px; color: #9d9d9d; }
          .navbar a { text-decoration: none; }
          .navbar a:hover, .navbar a:focus { color: #fff; }
          .navbar-header { font-size: 18px; }
          body > * { margin: 15px; padding: 0; }
          pre { padding: 10px; font-size: 13px; background-color: #f5f5f5; border: 1px solid #ccc; }
          h1, h2 { font-weight: 500; }
          a { color: #337ab7; }
          a:hover, a:focus { color: #23527c; }
        </style>
      </head>
      <body>
        <div class="navbar">
          <div class="navbar-header"><a href="/">alert-az-do</a></div>
          <div><a href="/config">Configuration</a></div>
          <div><a href="/metrics">Metrics</a></div>
          <div><a href="/debug/pprof">Profiling</a></div>
          <div><a href="{{ .DocsURL }}">Help</a></div>
        </div>
        {{template "content" .}}
      </body>
      </html>
    {{- end }}

    {{ define "content.home" -}}
      <p>This is <a href="{{ .DocsURL }}">alert-az-do</a>, a
        <a href="https://prometheus.io/docs/alerting/configuration/#webhook_config">webhook receiver</a> for
        <a href="https://prometheus.io/docs/alerting/alertmanager/">Prometheus Alertmanager</a>.
    {{- end }}

    {{ define "content.config" -}}
      <h2>Configuration</h2>
      <pre>{{ .Config }}</pre>
    {{- end }}

    {{ define "content.error" -}}
      <h2>Error</h2>
      <pre>{{ .Err }}</pre>
    {{- end }}
    `
)

type tdata struct {
	DocsURL string

	// `/config` only
	Config string

	// `/error` only
	Err error
}

var (
	allTemplates   = template.Must(template.New("").Parse(templates))
	homeTemplate   = pageTemplate("home")
	configTemplate = pageTemplate("config")
	// errorTemplate  = pageTemplate("error")
)

func pageTemplate(name string) *template.Template {
	pageTemplate := fmt.Sprintf(`{{define "content"}}{{template "content.%s" .}}{{end}}{{template "page" .}}`, name)
	return template.Must(template.Must(allTemplates.Clone()).Parse(pageTemplate))
}

func getScopes() []string {
	return []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"}
}

// HomeHandlerFunc is the HTTP handler for the home page (`/`).
func HomeHandlerFunc() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("only GET allowed"))
			return
		}

		if err := homeTemplate.Execute(w, &tdata{
			DocsURL: docsURL,
		}); err != nil {
			w.WriteHeader(500)
		}
	}
}

// ConfigHandlerFunc is the HTTP handler for the `/config` page. It outputs the configuration marshaled in YAML format.
func ConfigHandlerFunc(config *config.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("only GET allowed"))
			return
		}

		if err := configTemplate.Execute(w, &tdata{
			DocsURL: docsURL,
			Config:  config.String(),
		}); err != nil {
			w.WriteHeader(500)
		}
	}
}

func AlertHandlerFunc(ctx context.Context, logger log.Logger, config *config.Config, tmpl *tmpl.Template) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		level.Debug(logger).Log("msg", "handling /alert webhook request")
		defer func() { _ = req.Body.Close() }()

		// https://godoc.org/github.com/prometheus/alertmanager/template#Data
		data := alertmanager.Data{}
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			errorHandler(w, http.StatusBadRequest, err, unknownReceiver, &data, logger)
			return
		}

		conf := config.ReceiverByName(data.Receiver)
		if conf == nil {
			errorHandler(w, http.StatusNotFound, fmt.Errorf("receiver missing: %s", data.Receiver), unknownReceiver, &data, logger)
			return
		}
		level.Debug(logger).Log("msg", "  matched receiver", "receiver", conf.Name)

		// TODO:
		// SEE: https://github.com/AzureAD/microsoft-authentication-library-for-go/
		//var client azcore.Client
		var cred azcore.TokenCredential
		var err error
		if os.Getenv("AZURE_TENANT_ID") != "" && os.Getenv("AZURE_CLIENT_ID") != "" && conf.ClientSecret != "" {
			cred, err = azidentity.NewEnvironmentCredential(nil)
		} else if conf.TenantID != "" && conf.ClientID != "" && conf.ClientSecret != "" {
			cred, err = azidentity.NewClientSecretCredential(string(conf.TenantID), string(conf.ClientID), string(conf.ClientSecret), nil)
		} else if conf.PersonalAccessToken != "" {
			cred, err = azidentity.NewUsernamePasswordCredential("", "", "", string(conf.PersonalAccessToken), nil)
		}

		if err != nil {
			errorHandler(w, http.StatusInternalServerError, err, conf.Name, &data, logger)
			return
		}

		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: getScopes(),
		})
		if err != nil {
			level.Error(logger).Log("msg", "failed to create Azure DevOps client", "err", err)
			errorHandler(w, http.StatusInternalServerError, err, conf.Name, &data, logger)
		}

		conn := &v7.Connection{
			AuthorizationString: fmt.Sprintf("Bearer %s", token.Token),
			BaseUrl:             fmt.Sprintf("https://dev.azure.com/%s", conf.Organization),
		}

		if err := notify.NewReceiver(ctx, logger, conf, tmpl, conn).Notify(ctx, &data); err != nil {
			// Inaccurate, just letting Alertmanager know that it should not retry.
			errorHandler(w, http.StatusBadRequest, err, conf.Name, &data, logger)
			return
		}
		requestTotal.WithLabelValues(conf.Name, "200").Inc()
	}
}
