APP=concurget

build:
	go build -o $(APP)

run:
	go run . -f urls.txt -c 3

race:
	go run -race . -f urls.txt -c 3

clean:
	rm -f $(APP)