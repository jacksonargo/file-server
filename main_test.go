package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ContentRoot = "test"

func TestHandleGet(t *testing.T) {
	runTest := func(target string, wantStatus int, wantBody string) {
		httpRequest := httptest.NewRequest(http.MethodGet, target, nil)
		responseRecorder := httptest.NewRecorder()
		handleGet(ContentRoot, responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("file does not exist", func(t *testing.T) {
		runTest("/file_dne.txt", http.StatusNotFound, `{
          "status": "error",
          "type": "error",
          "error": {
            "code": 404,
            "error": "CreateFile test/file_dne.txt: The system cannot find the file specified."
          }
        }`)
	})

	t.Run("file exists", func(t *testing.T) {
		runTest("/file.txt", http.StatusOK, `{
          "status": "ok",
          "type": "file",
          "file_data": {
            "Name": "file.txt",
			"Path": "/file.txt",
            "Owner": "implement me",
            "Size": 6,
            "Permissions": 182,
            "Contents": "hello\n"
          }
        }`)
	})

	t.Run("root directory", func(t *testing.T) {
		runTest("/", http.StatusOK, `{
          "status": "ok",
          "type": "directory",
          "directory": {
            "Name": "/",
			"Path": "/",
            "Owner": "implement me",
            "Size": 0,
            "Permissions": 255,
            "Contents": [".hidden.txt", "file.txt", "somewhere"]
          }
        }`)
	})

	t.Run("directory", func(t *testing.T) {
		runTest("/somewhere", http.StatusOK, `{
          "status": "ok",
          "type": "directory",
          "directory": {
            "Name": "somewhere",
			"Path": "/somewhere",
            "Owner": "implement me",
            "Size": 0,
            "Permissions": 255,
            "Contents": ["another.txt"]
          }
        }`)
	})
}

func assertHttpResponse(t *testing.T, resp *http.Response, wantStatus int, wantBody string) {
	assertResponseHasHeader(t, resp, "Content-Type", "application/json")
	assertResponseHasStatusCode(t, resp, wantStatus)
	assertResponseHasBody(t, resp, wantBody)
}

func assertResponseHasHeader(t *testing.T, resp *http.Response, key, wantValue string) {
	gotValue := resp.Header.Get(key)
	if wantValue != gotValue {
		t.Errorf("unexpected value for header `%s`: want `%s`, got `%s`", key, wantValue, gotValue)
	}
}

func assertResponseHasStatusCode(t *testing.T, resp *http.Response, status int) {
	if want, got := resp.StatusCode, status; want != got {
		t.Errorf("unexpected status code: want `%v`, got `%v`", want, got)
	}
}

func assertResponseHasBody(t *testing.T, resp *http.Response, want string) {
	var gotResponse ResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&gotResponse); err != nil {
		t.Errorf("invalid response body: %v", err)
	}

	var wantResponse ResponseBody
	if err := json.NewDecoder(strings.NewReader(want)).Decode(&wantResponse); err != nil {
		t.Errorf("invalid json: %v", err)
	}

	assertEqualResponseBody(t, wantResponse, gotResponse)
}

func assertEqualResponseBody(t *testing.T, want, got ResponseBody) {
	wantJson, _ := json.MarshalIndent(want, "", "  ")
	gotJson, _ := json.MarshalIndent(got, "", "  ")
	if string(wantJson) != string(gotJson) {
		t.Errorf("unexpected response body:\nwant:\n%s\ngot:\n%s\n", wantJson, gotJson)
	}
}
