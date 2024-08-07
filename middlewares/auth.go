package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Limpid-LLC/saiService"
	_ "github.com/Limpid-LLC/saiService"
)

type Request struct {
	Method string      `json:"method"`
	Data   RequestData `json:"data"`
}

type RequestData struct {
	Microservice string `json:"microservice"`
	Method       string `json:"method"`
	Metadata     any    `json:"metadata"`
	Data         any    `json:"data"`
}

func CreateAuthMiddleware(authServiceURL string, microserviceName string, method string) func(next saiService.HandlerFunc, bodyData any, bodyMetadata any, requestGETData any) (any, int, error) {
	return func(next saiService.HandlerFunc, bodyData any, bodyMetadata any, requestGETData any) (any, int, error) {
		if authServiceURL == "" {
			log.Println("authMiddleware: auth service url is empty")
			return unauthorizedResponse("authServiceURL")
		}

		var dataMap map[string]any

		dataBytes, _ := json.Marshal(bodyData)

		_ = json.Unmarshal(dataBytes, &dataMap)

		if bodyMetadata == nil {
			log.Println("authMiddleware: bodyMetadata is nil")
			return unauthorizedResponse("empty bodyMetadata")
		}

		metadataMap := bodyMetadata.(map[string]any)

		if metadataMap["token"] == nil {
			log.Println("authMiddleware: bodyMetadata token is nil")
			return unauthorizedResponse("empty bodyMetadata token")
		}

		dataMap["token"] = metadataMap["token"]

		authReq := Request{
			Method: "check",
			Data: RequestData{
				Microservice: microserviceName,
				Method:       method,
				Data:         dataMap,
			},
		}

		jsonData, err := json.Marshal(authReq)
		if err != nil {
			log.Println("authMiddleware: error marshaling bodyData")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("marshaling -> " + err.Error())
		}

		req, err := http.NewRequest("POST", authServiceURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Println("authMiddleware: error creating request")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("creating request -> " + err.Error())
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("authMiddleware: error sending request to auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("sending request -> " + err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Println("authMiddleware: error reading body from auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("reading body -> " + err.Error())
		}

		var res map[string]string
		err = json.Unmarshal(body, &res)
		if err != nil {
			log.Println("authMiddleware: error unmarshalling body from auth")
			log.Println("authMiddleware: " + err.Error())
			return unauthorizedResponse("Unmarshal -> " + err.Error())
		}

		if res["result"] != "Ok" {
			log.Println("authMiddleware: response-body -> result is not `Ok`")
			log.Println("authMiddleware: " + string(body))
			return unauthorizedResponse("Result -> " + string(body))
		}

		return next(bodyData, bodyMetadata, requestGETData)
	}
}

func unauthorizedResponse(info string) (any, int, error) {
	return nil, http.StatusUnauthorized, errors.New("unauthorized:" + info)
}
