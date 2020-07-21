#Execute the following commands:
# 'make' and it will create the binary
# 'make clean' ant it will remove the binary
NAME:=atp-go

.PHONY: all build clean dep

all: dep build
build:
	go build -v ./...
dep: go.mod
	go mod tidy
	go get -v -u ./...
clean:
	go clean
	rm -f $(NAME)
go.mod:
	go mod init $(NAME)