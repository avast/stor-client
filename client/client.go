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
}

type DownPool struct {
	input  chan hashutil.Hash
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

var workerEnd hashutil.Hash = hashutil.Hash{}

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

	downloadPool := DownPool{
		input:  make(chan hashutil.Hash, 1024),
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
	for id := 0; id < client.max; id++ {
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
		total.Count++
	}

	totalStat <- total
}

// add sha to douwnload queue
func (client *StorClient) Download(sha hashutil.Hash) {
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
	for i := 0; i < client.max; i++ {
		client.pool.input <- workerEnd
	}
}

// format and log total stats
func (total TotalStat) Print(startTime time.Time) {
	var totalSizeMB float64 = (float64)(total.Size / (1024 * 1024))
	totalDuration := time.Since(startTime)

	log.Infof(
		"total downloaded size: %0.3fMB\ntotal time: %0.3fs\ndownload time: %0.3fs (sum of all downloads => unparallel)\ndownload rate %0.3fMB/s (unparallel rate %0.3fMB/s)\n",
		totalSizeMB,
		totalDuration.Seconds(),
		total.Duration.Seconds(),
		totalSizeMB/total.Duration.Seconds(),
		totalSizeMB/total.Duration.Seconds(),
	)
}
