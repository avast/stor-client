# stor-client

## what is stor?

[stor](https://github.com/avast/stor) is storage HTTP interface for sha256 files (objects)

## cli

```
echo EE2BF0BFD365EBF829F8D07B197B7A15F39760CD14C6D3BFDFBAD2B145CB72B8 | stor-client --storage http://stor.domain.tld .
```

## golang client

simple example

```
client := storclient.New(storageUrl, storclient.StorClientOpts{})

client.Start()

for _, sha := range shaList {
	client.Download(sha)
}

downloadStatus := client.Wait()
```
