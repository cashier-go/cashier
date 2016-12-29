## S3 plugin for WKFS



Package `s3` registers an AWS S3 filesystem at the well-known `/s3/` filesystem path.

Sample usage:

```go
package main

import (
	"fmt"
	"io"
	"log"

	"github.com/nsheridan/wkfs/s3"
	"go4.org/wkfs"
)

func main() {
  opts := &s3.Options{
    Region: "us-east-1"
    AccessKey: "abcdef"
    SecretKey: "secret"
  }
  s3.Register(opts)
  f, err := wkfs.Create("/s3/some-bucket/hello.txt")
  if err != nil {
    log.Fatal(err)
  }
  _, err := io.WriteString(f, "hello, world")
  if err != nil {
    log.Fatal(err)
  }
}
```



`Options` are completely optional as the AWS SDK will attempt to obtain credentials from a number of locations - see [the documentation for details](http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) - e.g. if you're using environment variables you can register the filesystem with `s3.Register(nil)`.
