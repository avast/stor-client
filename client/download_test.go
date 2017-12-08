package storclient

import (
	"crypto/sha256"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/JaSei/pathutil-go"
	"github.com/avast/hashutil-go"
	log "github.com/sirupsen/logrus"
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

type clientMockWithDelay struct {
	statusCode int
	status     string
}

func (c *clientMockWithDelay) Get(url string) (*http.Response, error) {
	var body bodyMock

	time.Sleep(time.Millisecond)

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
	_, err = downloadFileViaTempFile(client, path, "http://blabla", emptyHash)
	assert.NoError(t, err)
	assert.True(t, path.Exists(), "Downloaded file exists")
	assert.NoError(t, path.Remove())
}

func TestDownloadWorker(t *testing.T) {
	t.Run("File not found", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 404, status: "Not found"}
		oneDownloadWorkerTest(t, StorClientOpts{}, httpClient, emptyHash, func(tempdir pathutil.Path, stat DownStat) {
			assert.Equal(t, DOWN_FAIL, stat.Status)
			assert.Equal(t, int64(0), stat.Size)
		})
	})

	t.Run("lowercase", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		oneDownloadWorkerTest(t, StorClientOpts{}, httpClient, emptyHash, func(tempdir pathutil.Path, stat DownStat) {
			assert.Equal(t, DOWN_OK, stat.Status)
			assert.Equal(t, int64(0), stat.Size)

			downloadFile, err := tempdir.Child(strings.ToLower(emptyHash.String()))
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}
		})
	})

	t.Run("uppercase", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		oneDownloadWorkerTest(t, StorClientOpts{UpperCase: true}, httpClient, emptyHash, func(tempdir pathutil.Path, stat DownStat) {
			downloadFile, err := tempdir.Child(strings.ToUpper(emptyHash.String()))
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}

			assert.Equal(t, DOWN_OK, stat.Status)
			assert.Equal(t, int64(0), stat.Size)
		})
	})

	t.Run("extension", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		oneDownloadWorkerTest(t, StorClientOpts{UpperCase: true, Suffix: ".dat"}, httpClient, emptyHash, func(tempdir pathutil.Path, stat DownStat) {
			assert.Equal(t, DOWN_OK, stat.Status)
			assert.Equal(t, int64(0), stat.Size)

			downloadFile, err := tempdir.Child(strings.ToUpper(emptyHash.String()) + ".dat")
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}
		})
	})

	t.Run("more workers", func(t *testing.T) {
		httpClient := &clientMockWithDelay{statusCode: 200, status: "Ok"}
		downloadWorkersTest(t, StorClientOpts{}, httpClient, []hashutil.Hash{emptyHash, emptyHash}, 2, func(tempdir pathutil.Path, stats []DownStat) {
			assert.Equal(t, DOWN_SKIP, stats[0].Status)
			assert.Equal(t, DOWN_OK, stats[1].Status)

			downloadFile, err := tempdir.Child(emptyHash.String())
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}
		})
	})
}

func oneDownloadWorkerTest(t *testing.T, storClientOpts StorClientOpts, httpClient httpClient, sha256 hashutil.Hash, asserts func(pathutil.Path, DownStat)) {
	tempdir, err := pathutil.NewTempDir(pathutil.TempOpt{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, tempdir.RemoveTree())
	}()
	storClient := New(url.URL{}, tempdir.Canonpath(), storClientOpts)

	storClient.wg.Add(1)
	log.SetLevel(log.DebugLevel)

	shasForDownload := make(chan hashutil.Hash, 2)
	downloadedFilesStat := make(chan DownStat, 2)

	shasForDownload <- sha256
	shasForDownload <- workerEnd

	storClient.downloadWorker(0, httpClient, shasForDownload, downloadedFilesStat)

	stat := <-downloadedFilesStat
	asserts(tempdir, stat)
}

func downloadWorkersTest(t *testing.T, storClientOpts StorClientOpts, httpClient httpClient, sha256list []hashutil.Hash, workers int, asserts func(pathutil.Path, []DownStat)) {
	tempdir, err := pathutil.NewTempDir(pathutil.TempOpt{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, tempdir.RemoveTree())
	}()
	storClient := New(url.URL{}, tempdir.Canonpath(), storClientOpts)

	storClient.wg.Add(workers)
	log.SetLevel(log.DebugLevel)

	shasForDownload := make(chan hashutil.Hash, 3)
	downloadedFilesStat := make(chan DownStat, 3)

	for _, sha256 := range sha256list {
		shasForDownload <- sha256
	}

	shasForDownload <- workerEnd

	for i := 0; i < workers; i++ {
		go storClient.downloadWorker(0, httpClient, shasForDownload, downloadedFilesStat)
	}

	stats := make([]DownStat, workers)
	for i := 0; i < workers; i++ {
		stats[i] = <-downloadedFilesStat
	}
	asserts(tempdir, stats)
}
