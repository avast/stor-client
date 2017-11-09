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
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/avast/hashutil-go"
	log "github.com/sirupsen/logrus"
)

type StorClientOpts struct {
	//	max size of download pool
	Max int
	//	write to devnull instead of file
	Devnull bool
	//	connection timeout
	//
	//	-1 means no limit (no timeout)
	Timeout time.Duration
	// exponential retry - start delay time
	// default is 10e5 microseconds
	RetryDelay time.Duration
	// count of tries of retry
	// default is 10
	RetryTries uint
}

const (
	DefaultMax        = 4
	DefaultTimeout    = 30 * time.Second
	DefaultRetryTries = 10
	DefaultRetryDelay = 1e5 * time.Microsecond
)

type DownPool struct {
	input  chan hashutil.Hash
	output chan DownStat
}

type StorClient struct {
	downloadDir           string
	storageUrl            url.URL
	pool                  DownPool
	httpClient            *http.Client
	total                 chan TotalStat
	wg                    sync.WaitGroup
	expectedDownloadCount int
	StorClientOpts
}

type DownloadStatus int

const (
	// downlad fail (default)
	DOWN_FAIL DownloadStatus = iota
	// downlad skipped because file is downlad
	DOWN_SKIP
	// downlad ok
	DOWN_OK
)

type DownStat struct {
	Size     int64
	Duration time.Duration
	Status   DownloadStatus
}

// Size and Duration is duplicate, becuse embedding not works, because
//   https://stackoverflow.com/questions/41686692/embedding-structs-in-golang-gives-error-unknown-field
type TotalStat struct {
	Size     int64
	Duration time.Duration
	// Count of downloaded files
	Count int
	// Count of skipped files
	Skip                  int
	expectedDownloadCount int
}

var workerEnd hashutil.Hash = hashutil.Hash{}

// Create new instance of stor client
func New(storUrl url.URL, downloadDir string, opts StorClientOpts) *StorClient {
	client := StorClient{}

	client.storageUrl = storUrl
	client.downloadDir = downloadDir

	client.Max = DefaultMax
	if opts.Max != 0 {
		client.Max = opts.Max
	}

	client.Timeout = DefaultTimeout
	if opts.Timeout == -1 {
		client.Timeout = 0
	} else if opts.Timeout != 0 {
		client.Timeout = opts.Timeout
	}

	client.Devnull = opts.Devnull

	if opts.RetryDelay == 0 {
		client.RetryDelay = DefaultRetryDelay
	} else {
		client.RetryDelay = opts.RetryDelay
	}

	if opts.RetryTries == 0 {
		client.RetryTries = DefaultRetryTries
	} else {
		client.RetryTries = opts.RetryTries
	}

	downloadPool := DownPool{
		input:  make(chan hashutil.Hash, 1024),
		output: make(chan DownStat, 1024),
	}

	client.pool = downloadPool

	return &client
}

// start stor downloading process
func (client *StorClient) Start() {
	for id := 0; id < client.Max; id++ {
		client.wg.Add(1)
		go client.downloadWorker(id, client.pool.input, client.pool.output)
	}

	client.total = make(chan TotalStat, 1)
	go client.processStats(client.pool.output, client.total)
}

func (client *StorClient) processStats(downloadStats <-chan DownStat, totalStat chan<- TotalStat) {
	total := TotalStat{}
	for stat := range downloadStats {
		total.Size += stat.Size
		total.Duration += stat.Duration
		if stat.Status == DOWN_SKIP {
			total.Skip++
		} else if stat.Status == DOWN_OK {
			total.Count++
		}
	}

	total.expectedDownloadCount = client.expectedDownloadCount

	totalStat <- total
}

// add sha to douwnload queue
func (client *StorClient) Download(sha hashutil.Hash) {
	client.expectedDownloadCount++
	client.pool.input <- sha
}

// wait to all downloads
// return download stats
func (client *StorClient) Wait() TotalStat {
	client.sendEndSignalToAllWorkers()

	client.wg.Wait()
	close(client.pool.output)

	return <-client.total
}

func (client *StorClient) sendEndSignalToAllWorkers() {
	for i := 0; i < client.Max; i++ {
		client.pool.input <- workerEnd
	}
}

// format and log total stats
func (total TotalStat) Print(startTime time.Time) {
	var totalSizeMB float64 = (float64)(total.Size) / (1024 * 1024)
	totalDuration := time.Since(startTime)

	log.WithFields(log.Fields{
		"total download size":                 fmt.Sprintf("%0.3fMB", totalSizeMB),
		"total time":                          fmt.Sprintf("%0.3fs", totalDuration.Seconds()),
		"download rate":                       fmt.Sprintf("%0.3fMB/s", totalSizeMB/totalDuration.Seconds()),
		"expected count of files to download": total.expectedDownloadCount,
		"downloaded files":                    total.Count,
		"skipped files":                       total.Skip,
	}).Info("statistics")
}

// Status return true if all files are downloaded
func (total TotalStat) Status() bool {
	return total.Count+total.Skip == total.expectedDownloadCount
}
