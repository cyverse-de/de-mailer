all: de-mailer

de-mailer: test
	go build .

test:
	go test ./...

clean:
	rm -f de-mailer

.PHONY: clean test all
