package storclient

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/avast/retry-go"
	log "github.com/sirupsen/logrus"
)

type httpClient interface {
	Get(url string) (*http.Response, error)
}

func (client *StorClient) downloadWorker(shasForDownload <-chan string, downloadedFilesStat chan<- DownStat) {
	defer client.wg.Done()

	log.Debugln("Start download worker...")

	httpClient := client.newHttpClient()

	for sha := range shasForDownload {
		if sha == workerEnd {
			log.Debugln("worker end")
			return
		}

		filepath := path.Join(client.downloadDir, sha)

		if _, err := os.Stat(filepath); !os.IsNotExist(err) {
			log.Debugf("File %s exists - skip download", filepath)
			continue
		}

		startTime := time.Now()

		var size int64
		err := retry.RetryCustom(
			func() error {
				var err error
				size, err = downloadFile(httpClient, filepath, client.createUrl(sha), client.devnull, sha)

				return err
			},
			func(n uint, err error) {
				log.Debugf("Retry #%d: %s", n, err)
			},
			retry.NewRetryOpts(),
		)

		downloadDuration := time.Since(startTime)

		if err != nil {
			log.Errorf("Error download %s: %s\n", sha, err)
			downloadedFilesStat <- DownStat{}
		} else {
			log.Debugf("Downloaded %s\n", sha)
			downloadedFilesStat <- DownStat{Size: size, Duration: downloadDuration}
		}
	}
}

func (client *StorClient) newHttpClient() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:    client.max,
		IdleConnTimeout: client.timeout,
	}

	return &http.Client{Transport: tr}
}

func (client *StorClient) createUrl(sha string) string {
	storage := (client.storageUrl).String()
	storage = strings.TrimRight(storage, "/")

	return fmt.Sprintf("%s/%s", storage, sha)
}

func downloadFile(httpClient httpClient, filepath string, url string, devnull bool, expectedSha string) (size int64, err error) {
	var out interface{}

	if devnull {
		out = ioutil.Discard
	} else {
		out, err = os.Create(filepath)
		if err != nil {
			return 0, err
		}
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("Download fail %d (%s)", resp.StatusCode, resp.Status)
	}

	hasher := sha256.New()

	multi := io.MultiWriter(out.(io.Writer), hasher)

	size, err = io.Copy(multi, resp.Body)
	if err != nil {
		return 0, err
	}

	if !devnull {
		err := out.(*os.File).Close()
		if err != nil {
			return 0, err
		}
	}

	downSha256 := fmt.Sprintf("%x", hasher.Sum(nil))

	if strings.ToUpper(downSha256) != strings.ToUpper(expectedSha) {
		return 0, fmt.Errorf("Downloaded sha (%s) is not equal with expected sha (%s)", downSha256, expectedSha)
	}

	return size, nil
}
