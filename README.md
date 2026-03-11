# flowlog-go

Description

````
    Write a program that can parse file containing flow log data and filter rows based on source IP address, destination IP address.
    Provide documentation as well as test cases.
    Reference for flow log: https://docs.aws.amazon.com/vpc/latest/userguide/flow-log-records.html
````

Requirements:
````
    Input file is a plain text (ascii) file 
    The file size can be up to 20 MB
    The IP addresses are IPv4 only
````
Extensions
````
    Add support for filtering based on source and destination ports
    Store counts for connections. A connection is represented by a 5 tuple ( (source IP address, source port, destination IP address, destination port, transport protocol) 
````

Clone the repo

````
clone the repo
    git clone git@github.com:AherVaibhav/flowlog-go.git
````



DOCKER
````
cd into Directory
    cd flowlog-go

build the image
    docker build -t flowlog-go .

run port 8080
    docker run -p 8080:8080 flowlog-go

run different port
    docker run -p 9090:9090 flowlog-go -addr :9090

is it working
    curl http://localhost:8080/health

upload log file
    curl -X POST "http://localhost:8080/api/v1/flowlogs/parse?srcIp=10.0.1.5"  --data-binary @flows.log
````




Build GO
````
cd into directory
    cd flowlog-go

check go installed
    go version
    go version go1.26.1 darwin/arm64

set go env
    go env -w GO111MODULE=on

Verify
    go env GO111MODULE
    should print "ON"

Tidy
    go mod tidy

Tests

    go test ./... -count=1
    Sould procuce this
        ok  	github.com/flowlog/service/api/handler	0.581s
        ?   	github.com/flowlog/service/cmd/server	[no test files]
        ok  	github.com/flowlog/service/internal/filter	0.422s
        ?   	github.com/flowlog/service/internal/model	[no test files]
        ok  	github.com/flowlog/service/internal/parser	0.575s
        ?   	github.com/flowlog/service/pkg/middleware	[no test files]

    Check coverage
    go test ./... -v -cover 

Build
    go build -o flowlog-server ./cmd/server

Run
    ./flowlog-server -addr :8080


From another terminal call 
cd flowlog-go 

Call service

    curl http://localhost:8080/health

    curl -X POST "http://localhost:8080/api/v1/flowlogs/parse?filename=flows.log" --data-binary @flows.log

    curl -X POST "http://localhost:8080/api/v1/flowlogs/parse?srcIp=10.0.1.5" --data-binary @flows.log

    curl -X POST "http://localhost:8080/api/v1/flowlogs/parse?dstPort=53" --data-binary @flows.log

    curl -X POST "http://localhost:8080/api/v1/flowlogs/parse?srcIp=10.0.0.0%2F8&dstIp=10.0.0.0%2F8" --data-binary @flows.log
````