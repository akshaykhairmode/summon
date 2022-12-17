
# summon
Simple go tool to download file with multiple connections. Currently supports linux only.

**Requirements** - Go must be installed. go v1.6 and greater required. Download From https://golang.org/doc/install

**To install**, simply use  `go get github.com/akshaykhairmode/summon` or `go install github.com/akshaykhairmode/summon@latest` or `Download from dist folder`

This will install go binary in your $GOBIN (If its set) or at ~/go/bin/summon

**Example Usage** - `$GOBIN/summon -c 5 http://www.africau.edu/images/default/sample.pdf`

![Download Example](https://s9.gifyu.com/images/summon.gif)

**Flags Available**

     -c int
    	    number of concurrent connections
      -h    displays available flags
      -o string
            output path of downloaded file, default is same directory.
        


If you want to learn step by step, click [here](https://www.abilityrush.com/download-file-concurrently-in-golang-part-1/)
