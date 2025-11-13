// Copyright 2025 Stakater AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/jm-stakater/alert-az-do/pkg/config"
	v7 "github.com/microsoft/azure-devops-go-api/azuredevops/v7"

	_ "net/http/pprof"
)

type BasicCredential struct {
	Username string
	Password string
}

func NewBasicCredential(username, password string) (*BasicCredential, error) {
	if username == "" && password == "" {
		return nil, fmt.Errorf("both username and password cannot be empty for BasicCredential")
	}
	return &BasicCredential{
		Username: username,
		Password: password,
	}, nil
}

func (c *BasicCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	var t = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.Username, c.Password)))
	token := azcore.AccessToken{
		Token: t,
	}
	return token, nil
}

func getScopes() []string {
	return []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"}
}

func GetConnection(ctx context.Context, logger log.Logger, conf *config.ReceiverConfig) (*v7.Connection, error) {
	// Azure credential selection with proper authentication patterns
	cred, err := GetAuthenticationCredential(logger, conf)

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure DevOps client: %w", err)
	}

	authPrefix := "Bearer"
	if conf.PersonalAccessToken != "" {
		authPrefix = "Basic"
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: getScopes(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure DevOps client: %w", err)
	}

	conn := &v7.Connection{
		AuthorizationString: fmt.Sprintf("%s %s", authPrefix, token.Token),
		BaseUrl:             fmt.Sprintf("https://dev.azure.com/%s", conf.Organization),
	}

	return conn, nil
}

func GetAuthenticationCredential(logger log.Logger, conf *config.ReceiverConfig) (azcore.TokenCredential, error) {
	switch true {
	// Service Principal authentication (TenantID + ClientID + ClientSecret)
	case conf.TenantID != "" && conf.ClientID != "" && conf.ClientSecret != "" && conf.SubscriptionID == "" && conf.PersonalAccessToken == "":
		level.Debug(logger).Log("msg", "using Service Principal authentication")
		return azidentity.NewClientSecretCredential(string(conf.TenantID), string(conf.ClientID), string(conf.ClientSecret), nil)
		// Workload Identity authentication (ClientID + TenantID + Service Account Token)
	case conf.TenantID != "" && conf.ClientID != "" && conf.ClientSecret == "" && conf.SubscriptionID == "" && conf.PersonalAccessToken == "":
		level.Debug(logger).Log("msg", "using Workload Identity authentication")
		return azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			TenantID:      string(conf.TenantID),
			ClientID:      string(conf.ClientID),
			TokenFilePath: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		})
		// Managed Identity authentication (ClientID + SubscriptionID)
	case conf.TenantID == "" && conf.ClientID != "" && conf.ClientSecret == "" && conf.SubscriptionID != "" && conf.PersonalAccessToken == "":
		level.Debug(logger).Log("msg", "using Managed Identity authentication")
		return azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(string(conf.ClientID)),
		})
		// Personal Access Token (PAT) authentication
	case conf.TenantID == "" && conf.ClientID == "" && conf.ClientSecret == "" && conf.SubscriptionID == "" && conf.PersonalAccessToken != "":
		level.Debug(logger).Log("msg", "using Personal Access Token authentication")
		return NewBasicCredential("", string(conf.PersonalAccessToken))
	default:
		level.Debug(logger).Log("msg", "no valid authentication method configured", "config", conf)
		return nil, fmt.Errorf("no valid authentication method configured")
	}
}
