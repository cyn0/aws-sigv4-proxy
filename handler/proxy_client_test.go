/*
 * Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License").
 * You may not use this file except in compliance with the License.
 * A copy of the License is located at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
 * express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

type mockHTTPClient struct {
	Client
	Request *http.Request
	Fail    bool
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.Fail {
		return nil, fmt.Errorf("mockHTTPClient.Do failed")
	}
	m.Request = req
	return &http.Response{}, nil
}

type mockProvider struct {
	credentials.Provider
	Fail bool
}

func (m *mockProvider) Retrieve() (credentials.Value, error) {
	if m.Fail {
		return credentials.Value{}, fmt.Errorf("mockProvider.Retrieve failed")
	}
	return credentials.Value{}, nil
}

func TestProxyClient_Do(t *testing.T) {
	type want struct {
		resp    *http.Response
		request *http.Request
		err     error
	}

	tests := []struct {
		name        string
		request     *http.Request
		proxyClient *ProxyClient
		want        *want
	}{
		{
			name: "should fail if unable to build new request",
			request: &http.Request{
				Method: "💩💩💩💩💩",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`net/http: invalid method "💩💩💩💩💩"`),
			},
		},
		{
			name: "should fail if unable to determine service name",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`unable to determine service from host: badservice.host`),
			},
		},
		{
			name: "should use SignNameOverride and RegionOverride if provided",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client:              &mockHTTPClient{},
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "badservice.host",
				},
			},
		},
		{
			name: "should use HostOverride if provided",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "badservice.host",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client:              &mockHTTPClient{},
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
				HostOverride:        "host.override",
			},
			want: &want{
				resp:    &http.Response{},
				request: &http.Request{Host: "host.override"},
				err:     nil,
			},
		},
		{
			name: "should fail if unable to sign request",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`mockProvider.Retrieve failed`),
			},
		},
		{
			name: "should fail if unable to sign request",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{
					Fail: true,
				})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp:    nil,
				request: nil,
				err:     fmt.Errorf(`mockProvider.Retrieve failed`),
			},
		},
		{
			name: "should fail if request to target host fails",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{
					Fail: true,
				},
			},
			want: &want{
				resp: nil,
				err:  fmt.Errorf(`mockHTTPClient.Do failed`),
			},
		},
		{
			name: "should return request when everything 👍 (presign codepath)",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "s3.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "s3.amazonaws.com",
				},
			},
		},
		{
			name: "should return request when everything 👍",
			request: &http.Request{
				Method: "GET",
				URL:    &url.URL{},
				Host:   "execute-api.us-west-2.amazonaws.com",
				Body:   nil,
			},
			proxyClient: &ProxyClient{
				Signer: v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				Client: &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "execute-api.us-west-2.amazonaws.com",
				},
			},
		},
		{
			name: "should propagate non-zero content length",
			request: &http.Request{
				Method:        "PUT",
				URL:           &url.URL{},
				Host:          "not.important.host",
				ContentLength: 5,
				Body:          io.NopCloser(strings.NewReader("hello")),
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
				Client:              &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "not.important.host",
				},
			},
		},
		{
			name: "should propagate content length when it's zero",
			request: &http.Request{
				Method:        "PUT",
				URL:           &url.URL{},
				Host:          "not.important.host",
				ContentLength: 0,
				Body:          io.NopCloser(strings.NewReader("")),
			},
			proxyClient: &ProxyClient{
				Signer:              v4.NewSigner(credentials.NewCredentials(&mockProvider{})),
				SigningNameOverride: "ec2",
				RegionOverride:      "us-west-2",
				Client:              &mockHTTPClient{},
			},
			want: &want{
				resp: &http.Response{},
				err:  nil,
				request: &http.Request{
					Host: "not.important.host",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err :=
				tt.proxyClient.Do(tt.request)

			assert.Equal(t, tt.want.resp, resp)
			assert.Equal(t, tt.want.err, err)

			proxyRequest := tt.proxyClient.Client.(*mockHTTPClient).Request
			assert.True(t, verifyRequest(proxyRequest, tt.want.request))
			if proxyRequest == nil {
				return
			}

			// Ensure content length is propagated to the proxy request
			if proxyRequest != nil {
				assert.Equal(t, tt.request.ContentLength, proxyRequest.ContentLength)
			}

			// If this assertion is not true, then Go http client will use TransferEncoding: chunked, which
			// may not be supported by AWS services like S3.
			if tt.request.ContentLength == 0 {
				assert.Equal(t, http.NoBody, proxyRequest.Body)
			}
		})
	}
}

func verifyRequest(received *http.Request, expected *http.Request) bool {
	if expected == nil {
		return received == nil
	}

	return received.Host == expected.Host
}
