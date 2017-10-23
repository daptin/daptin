package resource

import (
	"strings"
	"github.com/artpar/resty"
	"fmt"
	"encoding/json"
)

type NetworkRequestActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *NetworkRequestActionPerformer) Name() string {
	return "$network.request"
}

func (d *NetworkRequestActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

	headers, isHeader := inFieldMap["Headers"]
	headerMap := make(map[string]string)
	if isHeader {
		headerMapInterface := headers.(map[string]interface{})
		for key, val := range headerMapInterface {
			headerMap[key] = val.(string)
		}

	}

	url, isUrlPresent := inFieldMap["Url"]
	var urlString string
	if isUrlPresent {
		urlString = url.(string)
	} else {
		return nil, []error{fmt.Errorf("URL not present in action attributes")}
	}

	body, isBody := inFieldMap["Body"]
	var bodyMap map[string]interface{}
	if isBody {
		bodyMap = body.(map[string]interface{})
	}

	formData, isFormData := inFieldMap["FormData"]
	formDataMap := make(map[string]string)
	if isFormData {
		formDataMapInterface := formData.(map[string]interface{})
		for key, val := range formDataMapInterface {
			formDataMap[key] = val.(string)
		}
	}

	queryParams, isQueryParams := inFieldMap["Query"]
	queryParamsMap := make(map[string]string)
	if isQueryParams {
		queryParamsMapInterface := queryParams.(map[string]interface{})
		for key, val := range queryParamsMapInterface {
			queryParamsMap[key] = val.(string)
		}
	}

	method, isMethodPresent := inFieldMap["Method"]
	if !isMethodPresent {
		return nil, []error{fmt.Errorf("Http Request method not present")}
	}
	methodString := strings.ToUpper(method.(string))

	client := resty.R()
	resty.DetectContentType(false)

	if isBody {
		client.SetBody(bodyMap)
	}
	if isFormData {
		client.SetFormData(formDataMap)
	}
	client.SetHeaders(headerMap)
	client.SetQueryParams(queryParamsMap)

	var response *resty.Response
	var err error

	//switch methodString {
	//case "get":
	//	response, err = client.Get(urlString)
	//case "post":
	//	response, err = client.Post(urlString)
	//case "put":
	//	response, err = client.Put(urlString)
	//case "patch":
	//	response, err = client.Patch(urlString)
	//case "delete":
	response, err = client.Execute(methodString, urlString)
	//}
	responseMap := make(map[string]interface{})

	responseHeaders := response.Header()
	responseContentType := responseHeaders.Get("Content-Type")
	if responseContentType == "application/json" {
		m := make(map[string]interface{})
		json.Unmarshal(response.Body(), &m)
		responseMap["body"] = m
	} else {
		responseMap["body"] = string(response.Body())
	}
	responseMap["headers"] = responseHeaders

	return []ActionResponse{{
		ResponseType: request.Type,
		Attributes:   responseMap,
	}}, []error{err}
}

func NewNetworkRequestPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := NetworkRequestActionPerformer{

	}

	return &handler, nil

}
