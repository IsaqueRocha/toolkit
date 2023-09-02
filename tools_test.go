package toolkit

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
)

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{jpegType, pngType}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedTypes: []string{jpegType, pngType}, renameFile: true, errorExpected: false},
	{name: "not allowed", allowedTypes: []string{jpegType}, renameFile: false, errorExpected: true},
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

	err := testTool.CreateDirIfNotExist("./testdata/mydir")
	assert.NoError(t, err)

	err = testTool.CreateDirIfNotExist("./testdata/mydir")
	assert.NoError(t, err)

	_ = os.Remove("./testdata/mydir")
}
