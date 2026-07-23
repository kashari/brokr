package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func executeHttpRequestAction(a *Action, ctxMap map[string]string, authToken string) (map[string]string, error) {
	url := interpolate(a.Url, ctxMap)

	var body io.Reader
	if len(a.Variables) > 0 {
		interpolated := make(map[string]string, len(a.Variables))
		for k, v := range a.Variables {
			interpolated[k] = interpolate(v, ctxMap)
		}
		b, err := json.Marshal(interpolated)
		if err != nil {
			return ctxMap, fmt.Errorf("marshal action body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(a.Method, url, body)
	if err != nil {
		return ctxMap, fmt.Errorf("build HTTP request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.ForwardToken && authToken != "" {
		req.Header.Set("Authorization", authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ctxMap, fmt.Errorf("HTTP action %s %s: %w", a.Method, url, err)
	}
	defer resp.Body.Close()

	if a.ExpectResponse && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return ctxMap, fmt.Errorf("HTTP action %s %s returned %d", a.Method, url, resp.StatusCode)
	}

	if a.ExpectResponse {
		var responseMap map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&responseMap); err == nil {
			for k, v := range responseMap {
				ctxMap[k] = v
			}
		}
	}

	return ctxMap, nil
}

// interpolate replaces ${varName} placeholders in s with values from ctxMap.
func interpolate(s string, ctxMap map[string]string) string {
	for k, v := range ctxMap {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

func executeSetContextMapAction(a *Action, ctxMap map[string]string) (map[string]string, error) {
	for k, v := range a.Variables {
		ctxMap[k] = interpolate(v, ctxMap)
	}
	return ctxMap, nil
}
