
# summon
Simple go tool to download file with multiple connections.

**Requirements** - Go must be installed. go v1.16 and greater required. Download From https://golang.org/doc/install

**To install**, simply use  `go get github.com/akshaykhairmode/summon` or `go install github.com/akshaykhairmode/summon@latest` or `Download from dist folder`

This will install go binary in your $GOBIN (If its set) or at ~/go/bin/summon

**Example Usage** - `$GOBIN/summon -c 5 https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf`

![Download Example](https://s9.gifyu.com/images/summon.gif)

**Flags Available**

      -c int
    	      number of concurrent connections
      -h    displays available flags
      -o string
            output path of downloaded file, default is same directory.
        
