package saiService

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Handler map[string]HandlerElement

type Middleware func(next HandlerFunc, bodyData any, bodyMetadata any, requestGETData any) (any, int, error)

type HandlerElement struct {
	Name        string
	Description string
	Function    HandlerFunc
	Middlewares []Middleware
}

type HandlerFunc = func(any, any, any) (any, int, error)

type JsonRequestType struct {
	Method         string         `json:"method"`
	BodyMetadata   map[string]any `json:"metadata"`
	BodyData       any            `json:"data"`
	RequestGETData any            `json:"-"`
}

type ErrorResponse map[string]any

func (s *Service) handleSocketConnections(conn net.Conn) {
	for {
		var message JsonRequestType
		socketMessage, _ := bufio.NewReader(conn).ReadString('\n')

		if socketMessage != "" {
			_ = json.Unmarshal([]byte(socketMessage), &message)

			if message.Method == "" {
				err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				_, _ = conn.Write(append(errBody, eos...))
				continue
			}

			result, _, resultErr := s.processPath(&message)

			if resultErr != nil {
				err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				_, _ = conn.Write(append(errBody, eos...))
				continue
			}

			body, marshalErr := json.Marshal(result)

			if marshalErr != nil {
				err := ErrorResponse{"Status": "NOK", "Error": marshalErr.Error()}
				errBody, _ := json.Marshal(err)
				log.Println(err)
				_, _ = conn.Write(append(errBody, eos...))
				continue
			}

			_, _ = conn.Write(append(body, eos...))
		}
	}
}

// handle cli command
func (s *Service) handleCliCommand(data []byte) ([]byte, error) {

	var message JsonRequestType
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data provided")
	}

	err := json.Unmarshal(data, &message)
	if err != nil {
		return nil, err
	}

	if message.Method == "" {
		return nil, fmt.Errorf("empty message method got")

	}

	result, _, err := s.processPath(&message)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Service) handleWSConnections(conn *websocket.Conn) {
	for {
		var message JsonRequestType
		if rErr := websocket.JSON.Receive(conn, &message); rErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			_ = websocket.JSON.Send(conn, err)
			continue
		}

		if message.Method == "" {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
			log.Println(err)
			_ = websocket.JSON.Send(conn, err)
			continue
		}

		headers := conn.Request().Header
		token := headers.Get("Token")
		if s.GetConfig("token", "").(string) != "" {
			if token != s.GetConfig("token", "") {
				err := ErrorResponse{"Status": "NOK", "Error": "Wrong token"}
				log.Println(err)
				_ = websocket.JSON.Send(conn, err)
				continue
			}
		}

		result, _, resultErr := s.processPath(&message)

		if resultErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
			log.Println(err)
			_ = websocket.JSON.Send(conn, err)
			continue
		}

		sErr := websocket.JSON.Send(conn, result)

		if sErr != nil {
			err := ErrorResponse{"Status": "NOK", "Error": sErr.Error()}
			log.Println(err)
			_ = websocket.JSON.Send(conn, err)
		}
	}
}

func (s *Service) healthCheck(resp http.ResponseWriter, req *http.Request) {
	data := map[string]any{"Status": "OK"}
	body, _ := json.Marshal(data)
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(body)
	return
}

func (s *Service) versionCheck(resp http.ResponseWriter, req *http.Request) {
	data := map[string]any{
		"Version": s.GetConfig("common.version", "0.1").(string),
		"Built":   s.GetBuild("no build date"),
	}
	body, _ := json.Marshal(data)
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(body)
	return
}

func (s *Service) handleHttpConnections(resp http.ResponseWriter, req *http.Request) {
	rc := http.NewResponseController(resp)

	writeTimeOut := s.GetConfig("common.http.write_timeout", 5).(int)
	err := rc.SetWriteDeadline(time.Now().Add(time.Duration(writeTimeOut) * time.Second))
	if err != nil {
		log.Println(err)
		return
	}

	readTimeOut := s.GetConfig("common.http.read_timeout", 5).(int)
	err = rc.SetReadDeadline(time.Now().Add(time.Duration(readTimeOut) * time.Second))
	if err != nil {
		log.Println(err)
		return
	}

	var message JsonRequestType
	decoder := json.NewDecoder(req.Body)
	decoderErr := decoder.Decode(&message)
	if message.BodyMetadata == nil {
		message.BodyMetadata = map[string]any{}
	}

	message.BodyMetadata["ip"] = s.getHttpIP(req)
	// Extract query parameters
	queryParams := req.URL.Query()
	requestGETData := map[string]any{}

	// Iterate through query parameters and print key-value pairs
	for key, values := range queryParams {
		requestGETData[key] = values
	}
	message.RequestGETData = requestGETData

	if decoderErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": decoderErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		_, _ = resp.Write(errBody)
		return
	}

	if message.Method == "" {
		err := ErrorResponse{"Status": "NOK", "Error": "Wrong message format"}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusBadRequest)
		_, _ = resp.Write(errBody)
		return
	}

	headers := req.Header
	token := headers.Get("Token")
	if s.GetConfig("common.token", "").(string) != "" {
		if token != s.GetConfig("common.token", "") {
			err := ErrorResponse{"Status": "NOK", "Error": "Wrong token"}
			errBody, _ := json.Marshal(err)
			log.Println(err)
			resp.WriteHeader(http.StatusUnauthorized)
			_, _ = resp.Write(errBody)
		}
	}

	result, statusCode, resultErr := s.processPath(&message)

	if statusCode == 210 {
		resp.Header().Set("Content-Type", "application/octet-stream")
		if resultErr != nil {
			resp.Header().Set("Content-Disposition", "attachment; filename="+resultErr.Error())
		}
		resp.WriteHeader(200)
		_, _ = resp.Write(result.([]byte))
		return
	}

	resp.Header().Set("Content-Type", "application/json")

	if resultErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": resultErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(statusCode)
		_, _ = resp.Write(errBody)
		return
	}

	body, marshalErr := json.Marshal(result)

	if marshalErr != nil {
		err := ErrorResponse{"Status": "NOK", "Error": marshalErr.Error()}
		errBody, _ := json.Marshal(err)
		log.Println(err)
		resp.WriteHeader(http.StatusInternalServerError)
		_, _ = resp.Write(errBody)
		return
	}
	resp.WriteHeader(statusCode)
	_, _ = resp.Write(body)
}

func (s *Service) applyMiddleware(handler HandlerElement, data any, metadata any, getData any) (any, int, error) {
	closures := make([]HandlerFunc, len(s.Middlewares)+len(handler.Middlewares)+1)
	closures[0] = handler.Function

	// Function to create a closure for the middleware with the correct next function
	createMiddlewareClosure := func(middleware Middleware, next HandlerFunc) HandlerFunc {
		return func(bodyData any, bodyMetadata any, requestGETData any) (any, int, error) {
			return middleware(next, bodyData, bodyMetadata, requestGETData)
		}
	}

	last := closures[0]

	// Apply global middlewares
	for _, middleware := range s.Middlewares {
		newClosure := createMiddlewareClosure(middleware, last)
		last = newClosure
		closures = append(closures, newClosure)
	}

	// Apply local middlewares
	for _, middleware := range handler.Middlewares {
		newClosure := createMiddlewareClosure(middleware, last)
		last = newClosure
		closures = append(closures, newClosure)
	}

	return last(data, metadata, getData)
}

func (s *Service) processPath(msg *JsonRequestType) (any, int, error) {
	h, ok := s.Handlers[msg.Method]

	if !ok {
		return nil, http.StatusNotFound, errors.New("no handler")
	}

	//todo: Routine per process

	// Apply middleware
	return s.applyMiddleware(h, msg.BodyData, msg.BodyMetadata, msg.RequestGETData)
}

func (s *Service) getHttpIP(r *http.Request) string {
	ip := r.Header.Get("X-REAL-IP")
	netIP := net.ParseIP(ip)
	if netIP != nil {
		return ip
	}

	ips := r.Header.Get("X-FORWARDED-FOR")
	splitIps := strings.Split(ips, ",")
	for _, ip := range splitIps {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			return ip
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}

	netIP = net.ParseIP(ip)
	if netIP != nil {
		return ip
	}

	return ""
}
