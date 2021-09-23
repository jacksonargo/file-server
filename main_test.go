package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
)

const ContentRoot = "test"

func TestHandleGet(t *testing.T) {
	runTest := func(t *testing.T, target string, wantStatus int, wantBody string) {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodGet, target, nil)
		responseRecorder := httptest.NewRecorder()
		httpHandler(ContentRoot).ServeHTTP(responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("file does not exist", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/file_dne.txt", http.StatusNotFound, `{
          "status": "error",
          "type": "error",
          "error": {
            "code": 404,
            "error": "stat test/file_dne.txt: no such file or directory"
          }
        }`)
	})

	t.Run("file exists", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustWriteFile(t, []byte("hello\n"), "/file.txt", 0644)
		runTest(t, "/file.txt", http.StatusOK, `{
          "status": "ok",
          "type": "file",
          "file": {
            "name": "file.txt",
			"path": "/file.txt",
            "owner": "0",
            "size": 6,
            "permissions": "0644",
            "contents": "hello\n"
          }
        }`)
	})

	t.Run("root directory", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustWriteFile(t, []byte("hello\n"), "/file.txt", 0644)
		mustWriteFile(t, []byte("hello\n"), "/.hidden.txt", 0644)
		mustMkDir(t, "/cheetos", 0755)
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
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustMkDir(t, "/cheetos", 0755)
		mustWriteFile(t, []byte("hello\n"), "/cheetos/file.txt", 0644)
		runTest(t, "/cheetos", http.StatusOK, `{
          "status": "ok",
          "type": "directory",
          "directory": {
            "name": "cheetos",
			"path": "/cheetos",
            "owner": "0",
            "size": 4096,
            "permissions": "0755",
            "entries": [
              {
                "name": "file.txt",
                "path": "/cheetos/file.txt",
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

func TestHandlePut(t *testing.T) {
	runTest := func(t *testing.T, target string, reqBody string, wantStatus int, wantBody string) {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodPut, target, strings.NewReader(reqBody))
		responseRecorder := httptest.NewRecorder()
		httpHandler(ContentRoot).ServeHTTP(responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("invalid json", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/",
			`{]`,
			http.StatusBadRequest,
			`{
			  "status": "error",
			  "type": "error",
			  "error": {
				"code": 400,
				"error": "invalid json: invalid character ']' looking for beginning of object key string"
			  }
        	}`)
	})

	t.Run("target is not a file", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustMkDir(t, "/dir", 0700)
		runTest(t, "/dir",
			`{"permissions": "0600", "contents": "hello\n"}`,
			http.StatusBadRequest,
			`{
			  "status": "error",
			  "type": "error",
			  "error": {
				"code": 400,
				"error": "test/dir is not a file"
			  }
        	}`)
	})

	t.Run("invalid permissions", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/new/file.txt",
			`{"permissions": "0x600", "contents": "hello\n"}`,
			http.StatusBadRequest,
			`{
			  "status": "error",
			  "type": "error",
			  "error": {
				"code": 400,
				"error": "test/new/file.txt has invalid octal permissions"
			  }
        	}`)
	})

	t.Run("success", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/new/file.txt",
			`{"permissions": "0600", "contents": "hello\n"}`,
			http.StatusOK,
			`{
			  "status": "ok",
			  "type": "file",
			  "file": {
				"name": "file.txt",
				"path": "/new/file.txt",
				"owner": "0",
				"permissions": "0600",
				"size": 6,
				"contents": "hello\n"
			  }
        	}`)
		assertFileContents(t, "/new/file.txt", 0600, "hello\n")
	})
}

func TestHandlePost(t *testing.T) {
	runTest := func(t *testing.T, target string, reqBody string, wantStatus int, wantBody string) {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodPost, target, strings.NewReader(reqBody))
		responseRecorder := httptest.NewRecorder()
		httpHandler(ContentRoot).ServeHTTP(responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("invalid json", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/",
			`{]`,
			http.StatusBadRequest,
			`{
			  "status": "error",
			  "type": "error",
			  "error": {
				"code": 400,
				"error": "invalid json: invalid character ']' looking for beginning of object key string"
			  }
        	}`)
	})

	t.Run("target is not a directory", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustWriteFile(t, []byte("hello\n"), "/file.txt", 0644)
		runTest(t, "/file.txt",
			`[{"name": "another_file.txt", "permissions": "0600", "contents": "hello\n"}]`,
			http.StatusBadRequest,
			`{
			  "status": "error",
			  "type": "error",
			  "error": {
				"code": 400,
				"error": "test/file.txt is not a directory"
			  }
        	}`)
	})

	t.Run("success", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/new/",
			`[{"name": "file.txt", "permissions": "0600", "contents": "hello\n"}]`,
			http.StatusOK,
			`{
			   "status": "ok",
			   "type": "directory",
			   "directory": {
				 "name": "new",
				 "path": "/new/",
				 "owner": "0",
				 "permissions": "0700",
				 "size": 4096,
				 "entries": [
				   {
					 "name": "file.txt",
					 "path": "/new/file.txt",
					 "owner": "0",
					 "permissions": "0600",
					 "size": 6,
					 "type": "file"
				   }
				 ]
			   }
			 }`)
		assertFileContents(t, "/new/file.txt", 0600, "hello\n")
	})
}

func TestHandleDelete(t *testing.T) {
	runTest := func(t *testing.T, target string, wantStatus int, wantBody string) {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodDelete, target, nil)
		responseRecorder := httptest.NewRecorder()
		httpHandler(ContentRoot).ServeHTTP(responseRecorder, httpRequest)
		assertHttpResponse(t, responseRecorder.Result(), wantStatus, wantBody)
	}

	t.Run("file does not exist", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		runTest(t, "/file.txt", http.StatusNotFound, `{
          "status": "error",
          "type": "error",
          "error": {
            "code": 404,
            "error": "remove test/file.txt: no such file or directory"
          }
        }`)
	})

	t.Run("delete file", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustWriteFile(t, []byte("hello\n"), "/file.txt", 0644)
		runTest(t, "/file.txt", http.StatusOK, `{"status":"ok","type":"deleted"}`)
		assertFileDoesNotExists(t, "/file.txt")
	})

	t.Run("delete empty directory", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustMkDir(t, "/new", 0700)
		runTest(t, "/new", http.StatusOK, `{"status":"ok","type":"deleted"}`)
		assertFileDoesNotExists(t, "/new")
	})

	t.Run("delete directory with contents", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustMkDir(t, "/new", 0700)
		mustWriteFile(t, []byte("hello\n"), "/new/file.txt", 0644)
		runTest(t, "/new", http.StatusBadRequest, `{
          "status": "error",
          "type": "error",
          "error": {
            "code": 400,
            "error": "remove test/new: directory not empty"
          }
        }`)
		assertFileExists(t, "/new")
	})

	t.Run("recursive delete directory", func(t *testing.T) {
		mustMakeContentRoot(t)
		defer mustDeleteContentRoot(t)

		mustMkDir(t, "/new", 0700)
		mustWriteFile(t, []byte("hello\n"), "/new/file.txt", 0644)
		runTest(t, "/new?recursive=true", http.StatusOK, `{"status":"ok","type":"deleted"}`)
		assertFileDoesNotExists(t, "/new")
	})
}

func mustMakeContentRoot(t *testing.T) {
	t.Helper()
	err := os.Mkdir(ContentRoot, 0700)
	switch {
	case err == nil:
		break
	case os.IsExist(err):
		break
	default:
		t.Fatal(err)
	}
}

func mustDeleteContentRoot(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(ContentRoot); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, data []byte, name string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path.Join(ContentRoot, name), data, perm); err != nil {
		t.Fatal(err)
	}
}

func mustMkDir(t *testing.T, name string, perm os.FileMode) {
	t.Helper()
	if err := os.Mkdir(path.Join(ContentRoot, name), perm); err != nil {
		t.Fatal(err)
	}
}

func assertFileExists(t *testing.T, target string) {
	t.Helper()
	_, err := os.Stat(path.Join(ContentRoot, target))
	if err != nil {
		t.Errorf("os.Stat() failed: `%v`", err)
		return
	}
}

func assertFileContents(t *testing.T, target string, wantPerms os.FileMode, wantContents string) {
	t.Helper()
	fileName := path.Join(ContentRoot, target)
	stat, err := os.Stat(fileName)
	if err != nil {
		t.Errorf("os.Stat() failed: %v", err)
		return
	}
	if want, got := wantPerms, stat.Mode().Perm(); want != got {
		t.Errorf("want perms %v, got perms %v", want, got)
	}

	gotContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("ioutil.ReadFile() failed: %v", err)
		return
	}
	if want, got := wantContents, string(gotContents); want != got {
		t.Errorf("want contents:\n%s\n,got content:\n%s\n", want, got)
	}
}

func assertFileDoesNotExists(t *testing.T, target string) {
	t.Helper()
	if _, err := os.Stat(path.Join(ContentRoot, target)); !os.IsNotExist(err) {
		t.Fatalf("wanted error like `%v`, got error `%v`", os.ErrNotExist, err)
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
