package oss

import (
	"github.com/denverdino/aliyungo/util"
	//"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const HeaderOSSPrefix = "x-oss-"

var ossParamsToSign = map[string]bool{
	"acl":                          true,
	"append":                       true,
	"bucketInfo":                   true,
	"cname":                        true,
	"comp":                         true,
	"cors":                         true,
	"delete":                       true,
	"endTime":                      true,
	"img":                          true,
	"lifecycle":                    true,
	"live":                         true,
	"location":                     true,
	"logging":                      true,
	"objectMeta":                   true,
	"partNumber":                   true,
	"position":                     true,
	"qos":                          true,
	"referer":                      true,
	"replication":                  true,
	"replicationLocation":          true,
	"replicationProgress":          true,
	"response-cache-control":       true,
	"response-content-disposition": true,
	"response-content-encoding":    true,
	"response-content-language":    true,
	"response-content-type":        true,
	"response-expires":             true,
	"security-token":               true,
	"startTime":                    true,
	"status":                       true,
	"style":                        true,
	"styleName":                    true,
	"symlink":                      true,
	"tagging":                      true,
	"uploadId":                     true,
	"uploads":                      true,
	"vod":                          true,
	"website":                      true,
	"x-oss-process":                true,
	"x-oss-traffic-limit":          true,
}

func (client *Client) signRequest(request *request) {
	query := request.params

	urlSignature := query.Get("OSSAccessKeyId") != ""

	headers := request.headers
	contentMd5 := headers.Get("Content-Md5")
	contentType := headers.Get("Content-Type")
	date := ""
	if urlSignature {
		date = query.Get("Expires")
	} else {
		date = headers.Get("Date")
	}

	resource := request.path
	if request.bucket != "" {
		resource = "/" + request.bucket + request.path
	}
	params := make(url.Values)
	for k, v := range query {
		if ossParamsToSign[k] {
			params[k] = v
		}
	}

	if len(params) > 0 {
		resource = resource + "?" + util.EncodeWithoutEscape(params)
	}

	canonicalizedResource := resource

	_, canonicalizedHeader := canonicalizeHeader(headers)

	stringToSign := request.method + "\n" + contentMd5 + "\n" + contentType + "\n" + date + "\n" + canonicalizedHeader + canonicalizedResource

	//log.Println("stringToSign: ", stringToSign)
	signature := util.CreateSignature(stringToSign, client.AccessKeySecret)

	if urlSignature {
		query.Set("Signature", signature)
	} else {
		headers.Set("Authorization", "OSS "+client.AccessKeyId+":"+signature)
	}
}

//Have to break the abstraction to append keys with lower case.
func canonicalizeHeader(headers http.Header) (newHeaders http.Header, result string) {
	var canonicalizedHeaders []string
	newHeaders = http.Header{}

	for k, v := range headers {
		if lower := strings.ToLower(k); strings.HasPrefix(lower, HeaderOSSPrefix) {
			newHeaders[lower] = v
			canonicalizedHeaders = append(canonicalizedHeaders, lower)
		} else {
			newHeaders[k] = v
		}
	}

	sort.Strings(canonicalizedHeaders)

	var canonicalizedHeader string

	for _, k := range canonicalizedHeaders {
		canonicalizedHeader += k + ":" + headers.Get(k) + "\n"
	}

	return newHeaders, canonicalizedHeader
}
