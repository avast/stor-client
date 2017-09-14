package storclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

type httpClient interface {
	Get(url string) (*http.Response, error)
}

func downloadFile(httpClient httpClient, filepath string, url string, devnull bool) (size int64, err error) {
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

	size, err = io.Copy(out.(io.Writer), resp.Body)
	if err != nil {
		return 0, err
	}

	if !devnull {
		err := out.(*os.File).Close()
		if err != nil {
			return 0, err
		}
	}

	return size, nil
}
