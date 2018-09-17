# stor-client

[![Release](https://img.shields.io/github/release/avast/stor-client.svg?style=flat-square)](https://github.com/avast/stor-client/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
[![Linux build status](https://img.shields.io/travis/avast/stor-client.svg?style=flat-square)](https://travis-ci.org/avast/stor-client)
[![Windows build status](https://ci.appveyor.com/api/projects/status/ab1v3564faurx8ad?svg=true)](https://ci.appveyor.com/project/JaSei/stor-client)
[![Go Report Card](https://goreportcard.com/badge/github.com/avast/stor-client?style=flat-square)](https://goreportcard.com/report/github.com/avast/stor-client)
[![GoDoc](https://godoc.org/github.com/avast/stor-client?status.svg&style=flat-square)](http://godoc.org/github.com/avast/stor-client)
[![Powered By: GoReleaser](https://img.shields.io/badge/powered%20by-goreleaser-green.svg?style=flat-square)](https://github.com/goreleaser)
[![codecov.io](https://codecov.io/github/avast/stor-client/coverage.svg?branch=master)](https://codecov.io/github/avast/stor-client?branch=master)
[![Sourcegraph](https://sourcegraph.com/github.com/avast/stor-client/-/badge.svg)](https://sourcegraph.com/github.com/avast/stor-client?badge)

## what is stor?

[stor](https://github.com/avast/stor) is storage HTTP interface for sha256 files (objects)

## features

* download retry
* concurent download (default `4`)

## cli

read (parse) SHA256 from STDIN and download it to `destinationDir`

```
echo EE2BF0BFD365EBF829F8D07B197B7A15F39760CD14C6D3BFDFBAD2B145CB72B8 | stor-client --storage http://stor.domain.tld .
```

### help

```
usage: stor-cli.exe [<flags>] <downloadDir>

Flags:
      --help           Show context-sensitive help (also try --help-long and --help-man).
  -u, --storage=http://stor.whale.int.avast.com storage url
      --max=4          max download process
      --devnull        download file to /dev/null
  -v, --verbose        more talkativ output
      --timeout=30s    connetion timeout
      --json           log in json format
      --delay=100ms    exponential retry - start delay time
      --attempts=10    count of attempts of retry
      --suffix=""      downloaded file suffix - like '.dat' => SHA.dat
      --upper          name of file will be upper case (not applied to suffix)
      --s3host=S3HOST  host to s3 endpoint with bucket e.g. https://bucket.s3.eu-central-1.amazonaws.com, if is s3url set, first will be use S3, then fallback to stor
      --s3template="{{.FirstShaByte}}/{{.SecondShaByte}}/{{.ThirdShaByte}}/{{.Sha}}" template to S3 path
      --version        Show application version.

Args:
  <downloadDir>  directory for downloaded files
```

## golang client

[golang stor-client library](client/README.md)

async example

```
client := storclient.New(storageUrl, storclient.StorClientOpts{})

client.Start()

for _, sha := range shaList {
	client.Download(sha)
}

downloadStatus := client.Wait()
```
