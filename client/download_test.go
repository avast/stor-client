package storclient

import (
	"crypto/sha256"
	"io"
	"net/http"
	"testing"

	"github.com/JaSei/pathutil-go"
	"github.com/avast/hashutil-go"
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

var emptyHash = hashutil.EmptyHash(sha256.New())

func TestDownloadFile(t *testing.T) {
	client := &clientMock{}

	_, err := downloadFileToDevnull(client, "http://blabla", emptyHash)
	assert.Error(t, err)

	client = &clientMock{statusCode: 200, status: "OK"}
	_, err = downloadFileToDevnull(client, "http://blabla", emptyHash)
	assert.NoError(t, err)

	path, err := pathutil.NewTempFile(pathutil.TempOpt{})
	assert.NoError(t, err)
	assert.NoError(t, path.Remove())

	client = &clientMock{statusCode: 200, status: "OK"}
	_, err = downloadFileViaTempFile(client, path.Canonpath(), "http://blabla", emptyHash)
	assert.NoError(t, err)
	assert.True(t, path.Exists(), "Downloaded file exists")
	assert.NoError(t, path.Remove())
}
