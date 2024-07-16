package internal

import (
	"github.com/Limpid-LLC/saiService"
)

func (is InternalService) NewHandler() saiService.Handler {
	return saiService.Handler{
		"get": saiService.HandlerElement{
			Name:        "get",
			Description: "Get value from the storage",
			Function: func(bodyData any, bodyMetadata any, requestGETData any) (any, int, error) {
				return is.get(bodyData)
			},
		},
		"post": saiService.HandlerElement{
			Name:        "post",
			Description: "Post value to the storage with specified key",
			Function: func(bodyData any, bodyMetadata any, requestGETData any) (any, int, error) {
				return is.post(bodyData)
			},
		},
	}
}

func (is InternalService) get(data any) (string, int, error) {
	return "Get:" + is.Context.GetConfig("test", "80").(string) + ":" + data.(string), 200, nil
}

func (is InternalService) post(data any) (string, int, error) {
	return "Post:" + is.Context.GetConfig("test", "80").(string) + ":" + data.(string), 200, nil
}
