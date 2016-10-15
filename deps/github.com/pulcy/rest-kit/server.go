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
	"encoding/json"
	"net/http"
)

// JSON creates a application/json content-type header, sets the given HTTP
// status code and encodes the given result object to the response writer.
func JSON(resp http.ResponseWriter, result interface{}, code int) error {
	resp.Header().Add("Content-Type", "application/json")
	resp.WriteHeader(code)
	if result != nil {
		return maskAny(json.NewEncoder(resp).Encode(result))
	}
	return nil
}

// Text creates a text/plain content-type header, sets the given HTTP
// status code and writes the given content to the response writer.
func Text(resp http.ResponseWriter, content string, code int) error {
	resp.Header().Add("Content-Type", "text/plain")
	resp.WriteHeader(code)
	_, err := resp.Write([]byte(content))
	return maskAny(err)
}

// Html creates a text/html content-type header, sets the given HTTP
// status code and writes the given content to the response writer.
func Html(resp http.ResponseWriter, content string, code int) error {
	resp.Header().Add("Content-Type", "text/html")
	resp.WriteHeader(code)
	_, err := resp.Write([]byte(content))
	return maskAny(err)
}

// Error sends an error message back to the given response writer.
func Error(resp http.ResponseWriter, err error) error {
	er := NewErrorResponseFromError(err)
	code := er.HTTPStatusCode()
	return maskAny(JSON(resp, er, code))
}
