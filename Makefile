.PHONY: all build run test clean docker-build

APP_NAME=loyalty-engine-service
MAIN_PATH=./cmd/server/main.go

all: build

build:
	go build -v -o bin/$(APP_NAME) $(MAIN_PATH)

run:
	go run $(MAIN_PATH)

test:
	go test -v -race ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t $(APP_NAME):latest .

