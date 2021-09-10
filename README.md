
# summon
Simple go tool to download file with multiple connections.

**Requirements** - Go must be installed. go v1.6 and greater reqeuired. Download From https://golang.org/doc/install

**To install**, simply use  `go get github.com/akshaykhairmode/summon`

This will install go binary in your $GOBIN (If its set) or at ~/go/bin/summon

**Example Usage** - `$GOBIN/summon -c 5 http://www.africau.edu/images/default/sample.pdf`

**Flags Available**
  
 

     -c int
    	    number of concurrent connections
      -h    displays available flags
      -o string
            output path of downloaded file, default is same directory.
        


**TODO**
 1. Ability to resume download.
