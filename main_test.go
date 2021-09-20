package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
)

const ContentRoot = "test"

func TestHandleGet(t *testing.T) {
	err := os.Mkdir(ContentRoot, 0700)
	switch {
	case err == nil:
		break
	case os.IsExist(err):
		break
	default:
		t.Fatal(err)
	}

	defer os.RemoveAll(ContentRoot)

	runTest := func(t *testing.T, target string, wantStatus int, wantBody string) {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodGet, target, nil)
		responseRecorder := httptest.NewRecorder()
		handleGet(ContentRoot, responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("file does not exist", func(t *testing.T) {
		runTest(t, "/file_dne.txt", http.StatusNotFound, `{
          "status": "error",
          "type": "error",
          "error": {
            "code": 404,
            "error": "lstat test/file_dne.txt: no such file or directory"
          }
        }`)
	})

	t.Run("file exists", func(t *testing.T) {
		dir := mustMakeTmpDir(t)
		defer mustDeleteAll(t, dir)

		mustWriteFile(t, []byte("hello\n"), dir+"/file.txt", 0644)
		runTest(t, dir[len(ContentRoot):]+"/file.txt", http.StatusOK, `{
          "status": "ok",
          "type": "file",
          "file": {
            "name": "file.txt",
			"path": "`+dir[len(ContentRoot):]+`/file.txt",
            "owner": "0",
            "size": 6,
            "permissions": "0644",
            "contents": "hello\n"
          }
        }`)
	})

	t.Run("root directory", func(t *testing.T) {
		mustWriteFile(t, []byte("hello\n"), ContentRoot+"/file.txt", 0644)
		defer mustDeleteAll(t, ContentRoot+"/file.txt")
		mustWriteFile(t, []byte("hello\n"), ContentRoot+"/.hidden.txt", 0644)
		defer mustDeleteAll(t, ContentRoot+"/.hidden.txt")
		mustMkDir(t, ContentRoot+"/cheetos", 0755)
		defer mustDeleteAll(t, ContentRoot+"/cheetos")
		runTest(t, "/", http.StatusOK, `{
          "status": "ok",
          "type": "directory",
          "directory": {
            "name": "/",
			"path": "/",
            "owner": "0",
            "size": 4096,
            "permissions": "0700",
            "entries": [
			  {
                "name": ".hidden.txt",
                "path": "/.hidden.txt",
                "owner": "0",
                "size": 6,
                "permissions": "0644",
				"type": "file"
              },
              {
                "name": "cheetos",
                "path": "/cheetos",
                "owner": "0",
                "size": 4096,
                "permissions": "0755",
				"type": "directory"
              },
              {
                "name": "file.txt",
                "path": "/file.txt",
                "owner": "0",
                "size": 6,
                "permissions": "0644",
				"type": "file"
              }
			]
          }
        }`)
	})

	t.Run("directory", func(t *testing.T) {
		dir := mustMakeTmpDir(t)
		defer mustDeleteAll(t, dir)

		mustWriteFile(t, []byte("hello\n"), dir+"/file.txt", 0644)

		runTest(t, dir[len(ContentRoot):], http.StatusOK, `{
          "status": "ok",
          "type": "directory",
          "directory": {
            "name": "`+path.Base(dir)+`",
			"path": "`+dir[len(ContentRoot):]+`",
            "owner": "0",
            "size": 4096,
            "permissions": "0700",
            "entries": [
              {
                "name": "file.txt",
                "path": "`+dir[len(ContentRoot):]+`/file.txt",
                "owner": "0",
                "permissions": "0644",
                "size": 6,
				"type": "file"
              }
			]
          }
        }`)
	})
}

func mustWriteFile(t *testing.T, data []byte, name string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, data, perm); err != nil {
		t.Fatal(err)
	}
}

func mustMkDir(t *testing.T, name string, perm os.FileMode) {
	t.Helper()
	if err := os.Mkdir(name, perm); err != nil {
		t.Fatal(err)
	}
}

func mustMakeTmpDir(t *testing.T) string {
	t.Helper()
	name := strings.Replace(t.Name(), "/", "_", -1)
	dir, err := os.MkdirTemp(ContentRoot, name)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustDeleteAll(t *testing.T, name string) {
	t.Helper()
	if err := os.RemoveAll(name); err != nil {
		t.Fatal(err)
	}
}

func assertHttpResponse(t *testing.T, resp *http.Response, wantStatus int, wantBody string) {
	t.Helper()
	assertResponseHasHeader(t, resp, "Content-Type", "application/json")
	assertResponseHasStatusCode(t, resp, wantStatus)
	assertResponseHasBody(t, resp, wantBody)
}

func assertResponseHasHeader(t *testing.T, resp *http.Response, key, wantValue string) {
	t.Helper()
	gotValue := resp.Header.Get(key)
	if wantValue != gotValue {
		t.Errorf("unexpected value for header `%s`: want `%s`, got `%s`", key, wantValue, gotValue)
	}
}

func assertResponseHasStatusCode(t *testing.T, resp *http.Response, status int) {
	t.Helper()
	if want, got := status, resp.StatusCode; want != got {
		t.Errorf("unexpected status code: want `%v`, got `%v`", want, got)
	}
}

func assertResponseHasBody(t *testing.T, resp *http.Response, want string) {
	t.Helper()
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
	t.Helper()
	wantJson, _ := json.MarshalIndent(want, "", "  ")
	gotJson, _ := json.MarshalIndent(got, "", "  ")
	if string(wantJson) != string(gotJson) {
		t.Errorf("unexpected response body:\nwant:\n%s\ngot:\n%s\n", wantJson, gotJson)
	}
}
