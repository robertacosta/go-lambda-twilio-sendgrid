.PHONY: deps clean build run

deps:
	go get -u ./...

clean:
	rm -rf artifacts/ev artifacts/ev.zip

build:	clean
	GOOS=linux GOARCH=amd64 go build -v -o artifacts/ev

run:	build
	sam local invoke "ev" -e artifacts/event.json -t artifacts/template.yaml --region us-east-2

zip:	build
	cd artifacts/ && zip ev.zip ev
