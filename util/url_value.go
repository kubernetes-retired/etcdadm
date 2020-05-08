/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"net/url"
)

// URLValue struct definition
type URLValue struct {
	URL *url.URL
}

// String reassembles the URL into a valid URL string
func (s URLValue) String() string {
	if s.URL != nil {
		return s.URL.String()
	}
	return ""
}

// Set sets the argument passed to URL Value
func (s URLValue) Set(val string) error {
	if u, err := url.Parse(val); err != nil {
		return err
	} else {
		*s.URL = *u
	}
	return nil
}

// Type returns type of struct
func (s URLValue) Type() string {
	return "URLValue"
}
