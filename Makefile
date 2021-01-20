#go tool dist list

build:
	go build -o gather main.go

compile:
	echo "Compiling for every OS and Platform"
	GOOS=darwin GOARCH=amd64 go build -o bin/darwin/gather main.go
	GOOS=freebsd GOARCH=386 go build -o bin/freebsd/gather main.go
	GOOS=linux GOARCH=386 go build -o bin/linux/gather main.go
	GOOS=windows GOARCH=386 go build -o bin/windows/gather main.go

run:
	go run main.go