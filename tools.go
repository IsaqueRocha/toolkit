package toolkit

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is the type used to instantiate this module. Any variable of this type
// will have access to all the methods with the receiver *Tools
type Tools struct {
	MaxFileSize      int64
	AllowedFileTypes []string
}

// RandomString returns a string of random characters of length n,
// using randomStringSource as the source for the string.
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

// UploadedFile is a struct used to store information about an uploaded file.
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	files, err := t.UploadFiles(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}
	return files[0], nil
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = 1024 * 1024 * 1024 // 1GB
	}

	err := r.ParseMultipartForm(t.MaxFileSize)
	if err != nil {
		return nil, errors.New("the uploaded file is too big")
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdr := range fHeaders {
			uploadedFiles, err = t.getUploadedFiles(uploadedFiles, hdr, uploadDir, renameFile)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

func (t *Tools) getUploadedFiles(uploadedFiles []*UploadedFile, hdr *multipart.FileHeader, uploadDir string, renameFile bool) ([]*UploadedFile, error) {
	infile, err := hdr.Open()
	if err != nil {
		return nil, err
	}
	defer infile.Close()

	buff := make([]byte, 512)
	_, err = infile.Read(buff)
	if err != nil {
		return nil, err
	}

	fileType := http.DetectContentType(buff)

	allowed := t.isAllowedFileType(fileType, t.AllowedFileTypes)

	if !allowed {
		return nil, errors.New("the uploaded file type is not permitted")
	}

	_, err = infile.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	uploadedFiles, err = t.renameUploadedFiles(uploadedFiles, hdr, uploadDir, infile, renameFile)
	if err != nil {
		return nil, err
	}

	return uploadedFiles, nil
}

func (t *Tools) isAllowedFileType(fileType string, allowedTypes []string) bool {
	if len(t.AllowedFileTypes) != 0 {
		return true
	}

	for _, x := range t.AllowedFileTypes {
		if strings.EqualFold(fileType, x) {
			return true
		}
	}
	return false
}

func (t *Tools) renameUploadedFiles(uploadedFiles []*UploadedFile, hdr *multipart.FileHeader, uploadDir string, infile multipart.File, renameFile bool) ([]*UploadedFile, error) {
	var uploadedFile UploadedFile

	if renameFile {
		uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdr.Filename))
	} else {
		uploadedFile.NewFileName = hdr.Filename
	}

	uploadedFile.OriginalFileName = hdr.Filename

	var outfile *os.File
	var err error
	defer outfile.Close()

	if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
		return nil, err
	} else {
		fileSize, err := io.Copy(outfile, infile)
		if err != nil {
			return nil, err
		}
		uploadedFile.FileSize = fileSize
	}

	uploadedFiles = append(uploadedFiles, &uploadedFile)

	return uploadedFiles, nil
}
