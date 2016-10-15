// Copyright (c) 2016 Epracom Advies.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package restkit

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	syspath "path"
	"strconv"
)

type RestClient struct {
	baseURL *url.URL

	RequestBuilder func(method, path string, query url.Values, reqBody interface{}) (*http.Request, error)
	ResultParser   func(resp *http.Response, body []byte, result interface{}) error
	ErrorParser    func(resp *http.Response, body []byte) error
	ResponseParser func(resp *http.Response, result interface{}) error
}

func NewRestClient(baseURL *url.URL) *RestClient {
	c := &RestClient{
		baseURL: baseURL,
	}
	c.RequestBuilder = c.DefaultRequestBuilder
	c.ResultParser = c.DefaultResultParser
	c.ErrorParser = c.DefaultErrorParser
	c.ResponseParser = c.DefaultResponseParser
	return c
}

// Request executes a client request.
// method: GET|POST|PUT|DELETE|HEAD
// path: Path relative to the path of the baseURL
// query: Query string (can be nil)
// reqBody: Object to marshal into the request body
// result: Reference to object to unmarshal the response into
func (c *RestClient) Request(method, path string, query url.Values, reqBody interface{}, result interface{}) error {
	req, err := c.RequestBuilder(method, path, query, reqBody)
	if err != nil {
		return maskAny(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return maskAny(err)
	}

	if err := c.ResponseParser(resp, result); err != nil {
		return maskAny(err)
	}
	return nil
}

// DefaultRequestBuilder implements the default RequestBuilder behavior.
func (c *RestClient) DefaultRequestBuilder(method, path string, query url.Values, reqBody interface{}) (*http.Request, error) {
	url := *c.baseURL
	url.Path = syspath.Join(url.Path, path)
	if query != nil {
		url.RawQuery = query.Encode()
	}

	var reqReader io.Reader
	var contentType string
	var contentLength string
	if reqBody != nil {
		content, err := json.Marshal(reqBody)
		if err != nil {
			return nil, maskAny(err)
		}
		reqReader = bytes.NewBuffer(content)
		contentType = "application/json"
		contentLength = strconv.Itoa(len(content))
	}

	req, err := http.NewRequest(method, url.String(), reqReader)
	if err != nil {
		return nil, maskAny(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentLength != "" {
		req.Header.Set("Content-Length", contentLength)
	}
	return req, nil
}

// DefaultResponseParser implements the default ResponseParser behavior.
// It reads the response body.
// Then if the status == OK, it tries to parse it into the result.
// Otherwise if the body is not empty, it tries to parse it into an ErrorResponse.
func (c *RestClient) DefaultResponseParser(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return maskAny(err)
	}

	if resp.StatusCode == http.StatusOK {
		if err := c.ResultParser(resp, body, result); err != nil {
			return maskAny(err)
		}
		return nil
	}

	if err := c.ErrorParser(resp, body); err != nil {
		return maskAny(err)
	}
	return nil
}

// DefaultErrorParser tries to parse the given response body into a the given result object.
func (c *RestClient) DefaultResultParser(resp *http.Response, body []byte, result interface{}) error {
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return maskAny(err)
		}
	}
	return nil
}

// DefaultErrorParser tries to parse the given response body into an ErrorResponse.
func (c *RestClient) DefaultErrorParser(resp *http.Response, body []byte) error {
	var er ErrorResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &er); err != nil {
			return maskAny(err)
		}
	} else {
		er.TheError.Message = resp.Status
	}
	er.statusCode = resp.StatusCode

	return maskAny(&er)
}
