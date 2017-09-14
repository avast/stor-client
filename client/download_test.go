package storclient

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type bodyMock string

func (b bodyMock) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (b bodyMock) Close() error {
	return nil
}

type clientMock struct {
	statusCode int
	status     string
}

func (c *clientMock) Get(url string) (*http.Response, error) {
	var body bodyMock

	return &http.Response{StatusCode: c.statusCode, Status: c.status, Body: body}, nil
}

//const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestDownloadFile(t *testing.T) {
	client := &clientMock{}

	_, err := downloadFile(client, "/tmp/a", "http://blabla", true, emptyHash)
	assert.Error(t, err)

	client = &clientMock{statusCode: 200, status: "OK"}
	_, err = downloadFile(client, "/tmp/a", "http://blabla", true, emptyHash)
	assert.NoError(t, err)

	client = &clientMock{statusCode: 200, status: "OK"}
	_, err = downloadFile(client, "/tmp/a", "http://blabla", false, emptyHash)
	assert.NoError(t, err)
}
