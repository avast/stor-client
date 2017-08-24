/* Client to download samples from stor service

SYNOPSIS

	client := storclient.New(storageUrl, storclient.StorClientOpts{})

	client.Start()

	for _, sha := range shaList {
		client.Download(sha)
	}

	downloadStatus := client.Wait()

*/
package storclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	log "github.com/sirupsen/logrus"
)

type StorClientOpts struct {
	Max     int
	Devnull bool
	//	connection timeout
	//
	//	-1 means no limit (no timeout)
	Timeout time.Duration
}

type DownPool struct {
	input  chan string
	output chan DownStat
}

type StorClient struct {
	max         int
	downloadDir string
	storageUrl  url.URL
	devnull     bool
	pool        DownPool
	httpClient  *http.Client
	timeout     time.Duration
	total       chan TotalStat
	wg          sync.WaitGroup
}

type DownStat struct {
	Size     int64
	Duration time.Duration
}

type TotalStat struct {
	DownStat
	Count int
}

const DefaultMax = 4
const DefaultTimeout = 30 * time.Second

// Create new instance of stor client
func New(storUrl url.URL, downloadDir string, opts StorClientOpts) *StorClient {
	client := StorClient{}

	client.storageUrl = storUrl
	client.downloadDir = downloadDir

	client.max = DefaultMax
	if opts.Max != 0 {
		client.max = opts.Max
	}

	client.timeout = DefaultTimeout
	if opts.Timeout == -1 {
		client.timeout = 0
	} else if opts.Timeout != 0 {
		client.timeout = opts.Timeout
	}

	client.devnull = opts.Devnull

	tr := &http.Transport{
		MaxIdleConns:    client.max,
		IdleConnTimeout: client.timeout,
	}
	client.httpClient = &http.Client{Transport: tr}

	downloadPool := DownPool{
		input:  make(chan string, 1024),
		output: make(chan DownStat, 1024),
	}

	client.pool = downloadPool

	return &client
}

func (client *StorClient) Max() int {
	return client.max
}

func (client *StorClient) Timeout() time.Duration {
	return client.timeout
}

// start stor downloading process
func (client *StorClient) Start() {
	for i := 0; i < client.max; i++ {
		client.wg.Add(1)
		go client.download(client.pool.input, client.pool.output)
	}

	client.total = make(chan TotalStat, 1)
	go client.processStat(client.pool.output, client.total)
}

// add sha to douwnload queue
func (client *StorClient) Download(sha string) {
	client.pool.input <- sha
}

// wait to all downloads
// return download stats
func (client *StorClient) Wait() TotalStat {
	for i := 0; i < client.max; i++ {
		client.pool.input <- ""
	}

	client.wg.Wait()
	close(client.pool.output)

	return <-client.total
}

func (client *StorClient) download(shasForDownload <-chan string, downloadedFilesStat chan<- DownStat) {
	log.Debugln("Start download worker...")

	defer client.wg.Done()

	for sha := range shasForDownload {
		if sha == "" {
			log.Debugln("worker end")
			return
		}

		filepath := path.Join(client.downloadDir, sha)

		storage := (client.storageUrl).String()
		storage = strings.TrimRight(storage, "/")

		url := fmt.Sprintf("%s/%s", storage, sha)

		startTime := time.Now()

		var size int64
		err := retry.Retry(
			func() error {
				var err error
				size, err = client.downloadFile(filepath, url)

				return err
			},
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

func (client *StorClient) downloadFile(filepath string, url string) (size int64, err error) {
	var out interface{}

	if client.devnull {
		out = ioutil.Discard
	} else {
		out, err = os.Create(filepath)
		if err != nil {
			return 0, err
		}
	}

	resp, err := client.httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("Download fail %d (%s)", resp.StatusCode, resp.Status)
	}

	size, err = io.Copy(out.(io.Writer), resp.Body)
	if err != nil {
		return 0, err
	}

	if !client.devnull {
		out.(*os.File).Close()
	}

	return size, nil
}

func (client *StorClient) processStat(downloadStats <-chan DownStat, totalStat chan<- TotalStat) {
	total := TotalStat{}
	for stat := range downloadStats {
		total.Size += stat.Size
		total.Duration += stat.Duration
		total.Count++
	}

	totalStat <- total
}

// format and print total stats
func (total TotalStat) Print(startTime time.Time) {
	var totalSizeMB float64 = (float64)(total.Size / (1024 * 1024))
	totalDuration := time.Since(startTime)

	fmt.Printf("total downloaded size: %0.3fMB\n", totalSizeMB)
	fmt.Printf("total time: %0.3fs\n", totalDuration.Seconds())
	fmt.Printf("download time: %0.3fs (sum of all downloads => unparallel)\n", total.Duration.Seconds())
	fmt.Printf("download rate %0.3fMB/s (unparallel rate %0.3fMB/s)\n", totalSizeMB/total.Duration.Seconds(), totalSizeMB/total.Duration.Seconds())
}
