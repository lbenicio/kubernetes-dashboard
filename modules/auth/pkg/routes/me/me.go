// Copyright 2017 The Kubernetes Authors.
//
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

package login

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v4"

	"k8s.io/dashboard/auth/pkg/oidc"
	"k8s.io/dashboard/client"
	"k8s.io/dashboard/client/args"
	"k8s.io/dashboard/errors"
	"k8s.io/dashboard/types"
)

const (
	tokenServiceAccountKey = "serviceaccount"
)

type ServiceAccount struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

func me(request *http.Request, response http.ResponseWriter) (*types.User, int, error) {
	return meWithRefresh(request, response, oidc.RefreshSession)
}

func meWithRefresh(request *http.Request, response http.ResponseWriter, refreshSession func(context.Context, *client.BrowserSession) (bool, error)) (*types.User, int, error) {
	if args.AnonymousAdminEnabled() {
		if _, err := request.Cookie("kd-session-0"); err == nil {
			client.ClearBrowserSessionCookie(response, request)
		}
		if _, err := request.Cookie("kd-oidc-state"); err == nil {
			client.ClearProtectedCookie(response, request, "kd-oidc-state")
		}
		k8sClient, err := client.Client(request)
		if err != nil {
			code, err := errors.HandleError(err)
			return nil, code, err
		}
		if _, err = k8sClient.Discovery().ServerVersion(); err != nil {
			code, err := errors.HandleError(err)
			return nil, code, err
		}
		return &types.User{Name: "Administrator", AuthMethod: "anonymous", Authenticated: true}, http.StatusOK, nil
	}

	if session, err := client.ReadBrowserSession(request); err == nil && session.OIDC {
		refreshed, refreshErr := refreshSession(request.Context(), session)
		if refreshErr != nil {
			client.ClearBrowserSessionCookie(response, request)
			return nil, http.StatusUnauthorized, errors.NewUnauthorized(errors.MsgLoginUnauthorizedError)
		}
		if refreshed {
			if err := client.SetBrowserSessionCookie(response, request, *session); err != nil {
				return nil, http.StatusInternalServerError, err
			}
		}
		request = request.Clone(request.Context())
		client.SetAuthorizationHeader(request, session.KubernetesToken)
	}

	k8sClient, err := client.Client(request)
	if err != nil {
		code, err := errors.HandleError(err)
		return nil, code, err
	}

	// Make sure that authorization token is valid
	if _, err = k8sClient.Discovery().ServerVersion(); err != nil {
		code, err := errors.HandleError(err)
		return nil, code, err
	}

	if session, err := client.ReadBrowserSession(request); err == nil {
		method := "token"
		name := session.Name
		if session.OIDC {
			method = "oidc"
		} else if name == "" {
			name = getUserFromToken(session.KubernetesToken).Name
		}
		return &types.User{
			Name:          name,
			Email:         session.Email,
			Picture:       session.Picture,
			Groups:        session.Groups,
			Scopes:        session.Scopes,
			AuthMethod:    method,
			Authenticated: true,
		}, http.StatusOK, nil
	}
	user := getUserFromToken(client.GetBearerToken(request))
	user.AuthMethod = "header"
	return user, http.StatusOK, nil
}

func getUserFromToken(token string) *types.User {
	parsed, _ := jwt.Parse(token, nil)
	if parsed == nil {
		return &types.User{Authenticated: true}
	}

	claims := parsed.Claims.(jwt.MapClaims)

	found, value := traverse(tokenServiceAccountKey, claims)
	if !found {
		return &types.User{Authenticated: true}
	}

	var sa ServiceAccount
	ok := transcode(value, &sa)
	if !ok {
		return &types.User{Authenticated: true}
	}

	return &types.User{Name: sa.Name, Authenticated: true}
}

func traverse(key string, m map[string]interface{}) (found bool, value interface{}) {
	for k, v := range m {
		if k == key {
			return true, v
		}

		if innerMap, ok := v.(map[string]interface{}); ok {
			return traverse(key, innerMap)
		}
	}

	return false, ""
}

func transcode(in, out interface{}) bool {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(in)
	if err != nil {
		return false
	}

	err = json.NewDecoder(buf).Decode(out)
	return err == nil
}
