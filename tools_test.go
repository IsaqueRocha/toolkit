package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestTools_PostJSONToRemote(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		// Test Request Parameters
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
			Header:     make(http.Header),
		}
	})

	var testTool Tools
	var foo struct {
		Bar string `json:"bar"`
	}
	foo.Bar = "baz"

	_, _, err := testTool.PushJSONToRemote("http://example.com/some/path", foo, client)
	assert.NoError(t, err)
}

func TestTools_RandomString(t *testing.T) {
	var testTools Tools
	s := testTools.RandomString(10)
	assert.Equal(t, 10, len(s))
}

const (
	jpegType    = "image/jpeg"
	pngType     = "image/png"
	filePath    = "./testdata/img.png"
	uploadsPath = "./testdata/uploads"
	testDir     = "./testdata/mydir"
)

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{
		name:          "allowed no rename",
		allowedTypes:  []string{jpegType, pngType},
		renameFile:    false,
		errorExpected: false,
	},
	{
		name:          "allowed rename",
		allowedTypes:  []string{jpegType, pngType},
		renameFile:    true,
		errorExpected: false,
	},
	{
		name:          "not allowed",
		allowedTypes:  []string{jpegType},
		renameFile:    false,
		errorExpected: true,
	},
}

func TestTools_UploadFiles(t *testing.T) {
	for _, e := range uploadTests {
		// set up a pipe to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			// create the form data filed 'file'
			part, err := writer.CreateFormFile("file", filePath)
			assert.NoError(t, err)
			assert.NotNil(t, part)

			f, err := os.Open(filePath)
			assert.NoError(t, err)
			assert.NotNil(t, f)

			defer f.Close()

			img, _, err := image.Decode(f)
			assert.NoError(t, err)
			assert.NotNil(t, img)

			err = png.Encode(part, img)
			assert.NoError(t, err)
		}()

		// read from the pipe which receives data
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		testTools := Tools{
			AllowedFileTypes: e.allowedTypes,
		}

		uploadedFiles, err := testTools.UploadFiles(request, uploadsPath, e.renameFile)

		if !e.errorExpected {
			assert.NoError(t, err)

			if _, err := os.Stat(fmt.Sprintf("%s/%s", uploadsPath, uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				assert.NoError(t, err)
			}

			// clean up
			_ = os.Remove(fmt.Sprintf("%s/%s", uploadsPath, uploadedFiles[0].NewFileName))
		}

		if e.errorExpected {
			assert.NoError(t, err)
		}

		wg.Wait()
	}
}

func TestTools_UploadOneFile(t *testing.T) {
	// set up a pipe to avoid buffering
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()

		// create the form data filed 'file'
		part, err := writer.CreateFormFile("file", filePath)
		assert.NoError(t, err)
		assert.NotNil(t, part)

		f, err := os.Open(filePath)
		assert.NoError(t, err)
		assert.NotNil(t, f)

		defer f.Close()

		img, _, err := image.Decode(f)
		assert.NoError(t, err)
		assert.NotNil(t, img)

		err = png.Encode(part, img)
		assert.NoError(t, err)
	}()

	// read from the pipe which receives data
	request := httptest.NewRequest("POST", "/", pr)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	testTools := Tools{
		AllowedFileTypes: []string{jpegType, pngType},
	}

	uploadedFile, err := testTools.UploadOneFile(request, uploadsPath, true)

	assert.NoError(t, err)

	if _, err := os.Stat(fmt.Sprintf("%s/%s", uploadsPath, uploadedFile.NewFileName)); os.IsNotExist(err) {
		assert.NoError(t, err)
	}

	// clean up
	_ = os.Remove(fmt.Sprintf("%s/%s", uploadsPath, uploadedFile.NewFileName))
}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var testTool Tools

	err := testTool.CreateDirIfNotExist(testDir)
	assert.NoError(t, err)

	err = testTool.CreateDirIfNotExist(testDir)
	assert.NoError(t, err)

	_ = os.Remove(testDir)

}

var slugTests = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{
		name:          "valid string",
		s:             "now is the time",
		expected:      "now-is-the-time",
		errorExpected: false,
	},
	{
		name:          "empty string",
		s:             "",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "complex string",
		s:             "now is the time for all GOOD men! + fish & such &^123",
		expected:      "now-is-the-time-for-all-good-men-fish-such-123",
		errorExpected: false,
	},
	{
		name:          "japanese string",
		s:             "こんにちば",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "japanese string and roman characters",
		s:             "こんにちば hello world",
		expected:      "hello-world",
		errorExpected: false,
	},
}

func TestTools_Slugify(t *testing.T) {
	var testTools Tools

	for _, e := range slugTests {
		t.Run(e.name, func(t *testing.T) {
			slug, err := testTools.Slugify(e.s)
			if e.errorExpected {
				assert.Error(t, err)
			} else {
				assert.Equal(t, e.expected, slug)
				assert.NoError(t, err)
			}
		})
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	var testTools Tools

	testTools.DownloadStaticFile(rr, req, "./testdata/", "pic.jpg", "puppy.jpg")

	result := rr.Result()
	defer result.Body.Close()

	assert.Equal(t, result.Header["Content-Length"][0], "98827")
	assert.Equal(t, result.Header["Content-Disposition"][0], "attachment; filename=\"puppy.jpg\"")

	_, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
}{
	{
		name:          "good json",
		json:          `{"foo": "bar"}`,
		errorExpected: false,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "badly formatted json",
		json:          `{"foo": }`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "incorrect type",
		json:          `{"foo": 1}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "two json files",
		json:          `{"foo": "1"}{"alpha":"beta"}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "empty body",
		json:          ``,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "syntax error",
		json:          `{"foo": 1"}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "unknown field",
		json:          `{"fooooo": "bar"}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "allow unknown field",
		json:          `{"fooooo": "bar"}`,
		errorExpected: false,
		maxSize:       1024,
		allowUnknown:  true,
	},
	{
		name:          "missing field name",
		json:          `{"jack": "bar"}`,
		errorExpected: false,
		maxSize:       1024,
		allowUnknown:  true,
	},
	{
		name:          "file too large",
		json:          `{"foo": "bar"}`,
		errorExpected: true,
		maxSize:       4,
		allowUnknown:  true,
	},
	{
		name:          "no json",
		json:          `hello world`,
		errorExpected: true,
		maxSize:       4,
		allowUnknown:  true,
	},
}

func TestTools_ReadJson(t *testing.T) {
	var testTools Tools

	for _, e := range jsonTests {
		t.Run(e.name, func(t *testing.T) {
			// set the max file size
			testTools.MaxJSONSize = e.maxSize
			// set the allow unknown
			testTools.AllowUnknownFields = e.allowUnknown
			// declare a var to read decoded json into
			var decodedJSON struct {
				Foo string `json:"foo"`
			}
			// create a request with the body
			req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(e.json)))
			assert.NoError(t, err)

			// create a recorder
			rr := httptest.NewRecorder()

			err = testTools.ReadJSON(rr, req, &decodedJSON)

			if e.errorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTools_WriteJSON(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()

	payload := JSONResponse{
		Error:   false,
		Message: "foo",
	}

	heraders := make(http.Header)
	heraders.Add("FOO", "BAR")
	err := testTools.WriteJSON(rr, http.StatusOK, payload, heraders)
	assert.NoError(t, err)
}

func TestTools_ErrorJSON(t *testing.T) {
	var testTools Tools

	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)
	assert.NoError(t, err)

	var payload JSONResponse
	decoder := json.NewDecoder(rr.Body)
	err = decoder.Decode(&payload)
	assert.NoError(t, err)
	assert.True(t, payload.Error)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
