/*
stor-client is command line utility for downloading from stor


what is stor?

Stor (https://github.com/avast/stor) is storage HTTP interface for sha256 files (objects).
Stor can help with step by step migration from posix storage to object storage (aka S3).

features

* download retry
* concurent download (default `4`)
* S3 download as primary place, stor as fallback

cli

read (parse) SHA256 from STDIN and download it to `destinationDir`

	echo EE2BF0BFD365EBF829F8D07B197B7A15F39760CD14C6D3BFDFBAD2B145CB72B8 | stor-client --storage http://stor.domain.tld .

golang client

look to github.com/avast/stor-client/client

*/
package main

import (
	"bufio"
	"crypto/sha256"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/avast/hashutil-go"
	"github.com/avast/stor-client/client"
	log "github.com/sirupsen/logrus"
)

var version = "master"

var (
	storageUrl    = kingpin.Flag("storage", "storage url").Short('u').Default("http://stor.whale.int.avast.com").URL()
	downloadDir   = kingpin.Arg("downloadDir", "directory for downloaded files").Required().String()
	max           = kingpin.Flag("max", "max download process").Default(strconv.Itoa(storclient.DefaultMax)).Int()
	devnull       = kingpin.Flag("devnull", "download file to /dev/null").Bool()
	verbose       = kingpin.Flag("verbose", "more talkativ output").Short('v').Bool()
	timeout       = kingpin.Flag("timeout", "connetion timeout").Default(storclient.DefaultTimeout.String()).Duration()
	logJson       = kingpin.Flag("json", "log in json format").Bool()
	retryDelay    = kingpin.Flag("delay", "exponential retry - start delay time").Default(storclient.DefaultRetryDelay.String()).Duration()
	retryAttempts = kingpin.Flag("attempts", "count of attempts of retry").Default(strconv.Itoa(storclient.DefaultRetryAttempts)).Uint()
	suffix        = kingpin.Flag("suffix", "downloaded file suffix - like '.dat' => SHA.dat").Default("").String()
	upperCase     = kingpin.Flag("upper", "name of file will be upper case (not applied to suffix)").Bool()
	s3url         = kingpin.Flag("s3host", "host to s3 endpoint with bucket e.g. https://bucket.s3.eu-central-1.amazonaws.com, if is s3url set, first will be use S3, then fallback to stor").URL()
	s3template    = kingpin.Flag("s3template", "template to S3 path").Default(storclient.DefaultS3Template).String()
)

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	if *verbose {
		log.SetLevel(log.DebugLevel)
	}

	if *logJson {
		log.SetFormatter(&log.JSONFormatter{})
	}

	startTime := time.Now()
	client, err := storclient.New(**storageUrl, *downloadDir, storclient.StorClientOpts{
		Max:           *max,
		Devnull:       *devnull,
		Timeout:       *timeout,
		RetryDelay:    *retryDelay,
		RetryAttempts: *retryAttempts,
		Suffix:        *suffix,
		UpperCase:     *upperCase,
		S3URL:         *s3url,
		S3Template:    *s3template,
	})
	if err != nil {
		log.Fatal(err)
	}
	client.Start()

	shas := readShaFromReader(os.Stdin)
	for shaHexStr := range shas {

		if hash, err := hashutil.StringToHash(sha256.New(), shaHexStr); err == nil {
			client.Download(hash)
		} else {
			log.Error("Invalid sha256: ", err)
		}
	}

	total := client.Wait()

	total.Print(startTime)

	if !total.Status() {
		os.Exit(1)
	}
}

func readShaFromReader(rd io.Reader) <-chan string {
	shas := make(chan string, 32)

	go func() {
		re := regexp.MustCompile("[a-fA-F0-9]{64}")
		scanner := bufio.NewScanner(rd)
		for scanner.Scan() {
			for _, sha := range re.FindStringSubmatch(scanner.Text()) {
				shas <- sha
			}
		}

		close(shas)
	}()

	return shas
}
