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

package alicloud

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

const baseURL = "http://100.100.100.200/latest"

type ECSMetadata struct {
	client *http.Client
}

// NewMetadata creates a new metadata instance
func NewECSMetadata() *ECSMetadata {
	return &ECSMetadata{
		client: &http.Client{},
	}
}

// GetMetadata try to get specific metadata on a ECS instance.
// Refer to https://help.aliyun.com/knowledge_detail/49122.html for more details.
func (e *ECSMetadata) GetMetadata(p string) (string, error) {
	httpURL := fmt.Sprintf("%s/meta-data/%s", baseURL, p)
	resp, err := e.client.Get(httpURL)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	return string(body), err
}
