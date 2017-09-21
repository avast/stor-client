package main

import (
	"bufio"
	"crypto/sha256"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/avast/hashutil-go"
	"github.com/avast/stor-client/client"
	log "github.com/sirupsen/logrus"
)

var version = "master"

var (
	storageUrl  = kingpin.Flag("storage", "storage url").Short('u').Default("http://stor.whale.int.avast.com").URL()
	downloadDir = kingpin.Arg("downloadDir", "directory for downloaded files").Required().String()
	max         = kingpin.Flag("max", "max download process").Default("4").Int()
	devnull     = kingpin.Flag("devnull", "download file to /dev/null").Bool()
	verbose     = kingpin.Flag("verbose", "more talkativ output").Short('v').Bool()
	timeout     = kingpin.Flag("timeout", "connetion timeout").Default(storclient.DefaultTimeout.String()).Duration()
	logJson     = kingpin.Flag("json", "log in json format").Bool()
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
	client := storclient.New(**storageUrl, *downloadDir, storclient.StorClientOpts{
		Max:     *max,
		Devnull: *devnull,
		Timeout: *timeout,
	})
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
