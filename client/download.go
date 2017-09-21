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

	"github.com/avast/hashutil-go"
	"github.com/avast/retry-go"
	log "github.com/sirupsen/logrus"
)

type httpClient interface {
	Get(url string) (*http.Response, error)
}

//type logFieldsError interface {
//	Error() string
//	LogFields() log.Fields
//}

type downloadError struct {
	sha        hashutil.Hash
	statusCode int
	status     string
}

func (err downloadError) Error() string {
	return fmt.Sprintf("Download of %s fail %d (%s)", err.sha, err.statusCode, err.status)
}

//func (err downloadError) LogFields() log.Fields {
//	return log.Fields{
//		"sha256":     err.sha.String(),
//		"statusCode": err.statusCode,
//		"status":     err.status,
//	}
//}

func (client *StorClient) downloadWorker(id int, shasForDownload <-chan hashutil.Hash, downloadedFilesStat chan<- DownStat) {
	defer client.wg.Done()

	log.WithField("worker", id).Debugln("Start download worker...")

	httpClient := client.newHttpClient()

	for sha := range shasForDownload {
		if sha.Equal(workerEnd) {
			log.WithField("worker", id).Debugln("worker end")
			return
		}

		filepath := path.Join(client.downloadDir, sha.String())

		if _, err := os.Stat(filepath); !os.IsNotExist(err) {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
			}).Debugf("File %s exists - skip download", filepath)

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
				log.WithFields(log.Fields{
					"worker": id,
					"sha256": sha.String(),
					//}).WithFields(err.(logFieldsError).LogFields()).Debugf("Retry #%d: %s", n, err)
				}).Debugf("Retry #%d: %s", n, err)
			},
			retry.NewRetryOpts(),
		)

		downloadDuration := time.Since(startTime)

		if err != nil {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
				"error":  err,
			}).Errorf("Error download %s: %s\n", sha, err)
			downloadedFilesStat <- DownStat{}
		} else {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
			}).Debugf("Downloaded %s", sha)
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

func (client *StorClient) createUrl(sha hashutil.Hash) string {
	storage := (client.storageUrl).String()
	storage = strings.TrimRight(storage, "/")

	return fmt.Sprintf("%s/%s", storage, sha)
}

func downloadFile(httpClient httpClient, filepath string, url string, devnull bool, expectedSha hashutil.Hash) (size int64, err error) {
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
		return 0, downloadError{sha: expectedSha, statusCode: resp.StatusCode, status: resp.Status}
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

	downSha256, err := hashutil.BytesToHash(sha256.New(), hasher.Sum([]byte{0}))
	if err != nil {
		return 0, err
	}

	if !downSha256.Equal(expectedSha) {
		return 0, fmt.Errorf("Downloaded sha (%s) is not equal with expected sha (%s)", downSha256, expectedSha)
	}

	return size, nil
}
