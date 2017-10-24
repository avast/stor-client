# storclient
--
    import "github.com/avast/stor-client/client"


    Client to download samples from stor service
### SYNOPSIS

    client := storclient.New(storageUrl, storclient.StorClientOpts{})

    client.Start()

    for _, sha := range shaList {
    	client.Download(sha)
    }

    downloadStatus := client.Wait()

## Usage

```go
const (
	DefaultMax        = 4
	DefaultTimeout    = 30 * time.Second
	DefaultRetryTries = 10
	DefaultRetryDelay = 1e5 * time.Microsecond
)
```

#### type DownPool

```go
type DownPool struct {
}
```


#### type DownStat

```go
type DownStat struct {
	Size     int64
	Duration time.Duration
}
```


#### type StorClient

```go
type StorClient struct {
	StorClientOpts
}
```


#### func  New

```go
func New(storUrl url.URL, downloadDir string, opts StorClientOpts) *StorClient
```
Create new instance of stor client

#### func (*StorClient) Download

```go
func (client *StorClient) Download(sha hashutil.Hash)
```
add sha to douwnload queue

#### func (*StorClient) Start

```go
func (client *StorClient) Start()
```
start stor downloading process

#### func (*StorClient) Wait

```go
func (client *StorClient) Wait() TotalStat
```
wait to all downloads return download stats

#### type StorClientOpts

```go
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
```


#### type TotalStat

```go
type TotalStat struct {
	DownStat
	// Count of downloaded files
	Count int
	// Count of skipped files
	Skip int
}
```


#### func (TotalStat) Print

```go
func (total TotalStat) Print(startTime time.Time)
```
format and log total stats

#### func (TotalStat) Status

```go
func (total TotalStat) Status() bool
```
Status return true if all files are downloaded
