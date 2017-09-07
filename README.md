# stor-client

[![Release](https://img.shields.io/github/release/avast/stor-client.svg?style=flat-square)](https://github.com/avast/stor-client/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
[![Travis](https://img.shields.io/travis/avast/stor-client.svg?style=flat-square)](https://travis-ci.org/avast/stor-client)
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

## golang client

async example

```
client := storclient.New(storageUrl, storclient.StorClientOpts{})

client.Start()

for _, sha := range shaList {
	client.Download(sha)
}

downloadStatus := client.Wait()
```
