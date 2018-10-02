package storclient

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JaSei/pathutil-go"
	"github.com/avast/hashutil-go"
	"github.com/avast/retry-go"
	"github.com/pkg/errors"
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

type successDownload struct {
	size         int64
	lastModified time.Time
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

func (client *StorClient) downloadWorker(id int, httpClientFunc func() httpClient, shasForDownload <-chan hashutil.Hash, downloadedFilesStat chan<- DownStat) {
	defer client.wg.Done()

	log.WithField("worker", id).Debugln("Start download worker...")

	for sha := range shasForDownload {
		if sha.Equal(workerEnd) {
			log.WithField("worker", id).Debugln("worker end")
			return
		}

		filename := sha.String()
		if client.UpperCase {
			filename = strings.ToUpper(sha.String())
		}

		filename += client.Suffix

		filepath, err := pathutil.New(client.downloadDir, filename)
		if err != nil {
			log.Errorf("path problem: %s", err)

			downloadedFilesStat <- DownStat{Status: DOWN_FAIL}

			continue
		}

		if filepath.Exists() {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
			}).Debugf("File %s exists - skip download", filepath)

			downloadedFilesStat <- DownStat{Status: DOWN_SKIP}

			continue
		}

		if !client.currentDownloads.ContainsOrAdd(sha) {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
			}).Debug("File is now downloading in other worker - skip download")

			downloadedFilesStat <- DownStat{Status: DOWN_SKIP}

			continue
		}

		startTime := time.Now()

		tryS3 := false
		if client.S3URL != nil {
			tryS3 = true
		}

		var size int64
		err = retry.Do(
			func() error {
				var err error

				var u string
				if tryS3 {
					var urlErr error
					u, urlErr = client.createS3URL(sha)
					if urlErr != nil {
						log.WithFields(log.Fields{
							"worker": id,
							"sha256": sha.String(),
						}).Warningf("S3 template fail: %s", urlErr)
					} else {
						log.WithFields(log.Fields{
							"worker": id,
							"sha256": sha.String(),
						}).Debugf("Use S3 url %s", u)
					}
				}
				if u == "" {
					u = client.createStorURL(sha)
					log.WithFields(log.Fields{
						"worker": id,
						"sha256": sha.String(),
					}).Debugf("Use Stor url %s", u)
				}

				if client.Devnull {
					size, err = downloadFileToDevnull(httpClientFunc(), u, sha)
				} else {
					size, err = downloadFileViaTempFile(httpClientFunc(), filepath, u, sha)
				}

				return err
			},
			retry.OnRetry(func(n uint, err error) {
				log.WithFields(log.Fields{
					"worker": id,
					"sha256": sha.String(),
				}).Debugf("Retry #%d: %s", n, err)
			}),
			retry.RetryIf(func(err error) bool {
				switch e := err.(type) {
				case downloadError:
					if (downloadError)(e).statusCode == 404 && tryS3 {
						tryS3 = false
					} else if (downloadError)(e).statusCode == 404 {
						return false
					}
				}

				return true
			}),
			retry.Delay(client.RetryDelay),
			retry.Attempts(client.RetryAttempts),
			retry.Units(1),
		)

		downloadDuration := time.Since(startTime)
		client.currentDownloads.Del(sha)

		if err != nil {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
				"error":  err,
			}).Errorf("Error download %s: %s\n", sha, err)
			downloadedFilesStat <- DownStat{Status: DOWN_FAIL}
		} else {
			log.WithFields(log.Fields{
				"worker": id,
				"sha256": sha.String(),
			}).Debugf("Downloaded %s", sha)
			downloadedFilesStat <- DownStat{Size: size, Duration: downloadDuration, Status: DOWN_OK}
		}
	}
}

func (client *StorClient) newHTTPClient() httpClient {
	tr := &http.Transport{
		MaxIdleConns:    client.Max,
		IdleConnTimeout: client.Timeout,
	}

	return &http.Client{Transport: tr}
}

func (client *StorClient) createS3URL(sha hashutil.Hash) (string, error) {
	var pathBytes bytes.Buffer
	shaStr := sha.String()
	params := struct{ Sha, FirstShaByte, SecondShaByte, ThirdShaByte string }{shaStr, shaStr[0:2], shaStr[2:4], shaStr[4:6]}
	if err := client.s3template.Execute(&pathBytes, params); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", client.S3URL.String(), pathBytes.String()), nil
}

func (client *StorClient) createStorURL(sha hashutil.Hash) string {
	storage := (client.storageUrl).String()
	storage = strings.TrimRight(storage, "/")
	return fmt.Sprintf("%s/%s", storage, sha)
}

func downloadFileToDevnull(httpClient httpClient, url string, expectedSha hashutil.Hash) (size int64, err error) {
	succ, err := downloadFileToWriter(httpClient, url, ioutil.Discard, expectedSha)
	return succ.size, err
}

func downloadFileViaTempFile(httpClient httpClient, filepath pathutil.Path, url string, expectedSha hashutil.Hash) (size int64, err error) {
	temppath, err := pathutil.NewTempFile(pathutil.TempOpt{Dir: filepath.Parent().Canonpath(), Prefix: fmt.Sprintf("%s_*.temp", expectedSha)})
	if err != nil {
		return 0, errors.Wrap(err, "Construct of new temp file fail")
	}

	// cleanup tempfile if this function fail (err is set)
	defer func() {
		if err != nil {
			if remErr := temppath.Remove(); remErr != nil {
				err = errors.Wrapf(remErr, "Cleanup tempfile %s fail", temppath)
			}
		}
	}()

	if temppath.Exists() {
		if err := temppath.Remove(); err != nil {
			return 0, errors.Wrapf(err, "Cleanup old (exists) tempfile %s fail", temppath)
		}
	}

	succ, err := downloadFile(httpClient, temppath, url, expectedSha)
	if err != nil {
		return 0, err
	}

	if _, err := temppath.Rename(filepath.Canonpath()); err != nil {
		return 0, errors.Wrapf(err, "Rename temp %s to final path %s fail", temppath, filepath)
	}

	if err = os.Chtimes(filepath.Canonpath(), succ.lastModified, succ.lastModified); err != nil {
		return 0, errors.Wrapf(err, "Chtimes(%s, %s) fail", filepath.Canonpath(), succ.lastModified.String())
	}

	return succ.size, nil
}

func downloadFile(httpClient httpClient, path pathutil.Path, url string, expectedSha hashutil.Hash) (succ successDownload, err error) {
	out, err := path.OpenWriter()
	if err != nil {
		return successDownload{}, errors.Wrapf(err, "OpenWriter to tempfile %s fail", path)
	}

	defer func() {
		if errClose := out.Close(); errClose != nil {
			err = errors.Wrapf(err, "Close %s fail", path)
		}
	}()

	return downloadFileToWriter(httpClient, url, out, expectedSha)
}

func downloadFileToWriter(httpClient httpClient, url string, out io.Writer, expectedSha hashutil.Hash) (succ successDownload, err error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return successDownload{}, err
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			err = errClose
		}
	}()

	if resp.StatusCode != 200 {
		return successDownload{}, downloadError{sha: expectedSha, statusCode: resp.StatusCode, status: resp.Status}
	}

	lastModified, err := getLastModifiedTime(resp)
	if err != nil {
		return successDownload{}, err
	}

	hasher := sha256.New()
	multi := io.MultiWriter(out, hasher)

	size, err := io.Copy(multi, resp.Body)
	if err != nil {
		return successDownload{}, err
	}

	downSha256, err := hashutil.BytesToHash(sha256.New(), hasher.Sum(nil))
	if err != nil {
		return successDownload{}, err
	}

	if !downSha256.Equal(expectedSha) {
		return successDownload{}, fmt.Errorf("Downloaded sha (%s) is not equal with expected sha (%s)", downSha256, expectedSha)
	}

	return successDownload{
		size:         size,
		lastModified: lastModified,
	}, nil
}

func getLastModifiedTime(resp *http.Response) (time.Time, error) {
	lastModified := time.Now()
	var err error

	if lastModifiedStr := resp.Header.Get("Last-Modified"); lastModifiedStr != "" {
		log.Info(lastModifiedStr)
		lastModified, err = http.ParseTime(lastModifiedStr)
		if err != nil {
			return lastModified, err
		}
	}

	return lastModified, nil
}
