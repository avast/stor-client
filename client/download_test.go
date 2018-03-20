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
	header     http.Header
}

func (c *clientMock) Get(url string) (*http.Response, error) {
	var body bodyMock

	return &http.Response{StatusCode: c.statusCode, Status: c.status, Body: body, Header: c.header}, nil
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
		downloadWorkersTest(t, StorClientOpts{}, httpClient, []hashutil.Hash{emptyHash}, 1, func(tempdir pathutil.Path, stat []DownStat) {
			assert.Equal(t, DOWN_FAIL, stat[0].Status)
			assert.Equal(t, int64(0), stat[0].Size)
		})
	})

	t.Run("lowercase", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		downloadWorkersTest(t, StorClientOpts{}, httpClient, []hashutil.Hash{emptyHash}, 1, func(tempdir pathutil.Path, stat []DownStat) {
			assert.Equal(t, DOWN_OK, stat[0].Status)
			assert.Equal(t, int64(0), stat[0].Size)

			downloadFile, err := tempdir.Child(strings.ToLower(emptyHash.String()))
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}
		})
	})

	t.Run("uppercase", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		downloadWorkersTest(t, StorClientOpts{UpperCase: true}, httpClient, []hashutil.Hash{emptyHash}, 1, func(tempdir pathutil.Path, stat []DownStat) {
			downloadFile, err := tempdir.Child(strings.ToUpper(emptyHash.String()))
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}

			assert.Equal(t, DOWN_OK, stat[0].Status)
			assert.Equal(t, int64(0), stat[0].Size)
		})
	})

	t.Run("extension", func(t *testing.T) {
		httpClient := &clientMock{statusCode: 200, status: "Ok"}
		downloadWorkersTest(t, StorClientOpts{UpperCase: true, Suffix: ".dat"}, httpClient, []hashutil.Hash{emptyHash}, 1, func(tempdir pathutil.Path, stat []DownStat) {
			assert.Equal(t, DOWN_OK, stat[0].Status)
			assert.Equal(t, int64(0), stat[0].Size)

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

	t.Run("Last-Modified header", func(t *testing.T) {
		header := http.Header{}
		header.Add("Last-Modified", "Tue, 20 Mar 2018 15:48:42 GMT")
		httpClient := &clientMock{statusCode: 200, status: "Ok", header: header}
		downloadWorkersTest(t, StorClientOpts{}, httpClient, []hashutil.Hash{emptyHash}, 1, func(tempdir pathutil.Path, stat []DownStat) {
			assert.Equal(t, DOWN_OK, stat[0].Status)
			assert.Equal(t, int64(0), stat[0].Size)

			downloadFile, err := tempdir.Child(strings.ToLower(emptyHash.String()))
			assert.NoError(t, err)

			if !assert.True(t, downloadFile.Exists()) {
				t.Log(tempdir.Children())
			}

			st, err := downloadFile.Stat()
			assert.NoError(t, err)
			assert.True(t, time.Date(2018, time.March, 20, 15, 48, 42, 0, time.UTC).Equal(st.ModTime()))
		})
	})
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
