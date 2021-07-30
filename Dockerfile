FROM golang:1.14-alpine

RUN apk add --no-cache git
RUN go get -u github.com/jstemmer/go-junit-report

ENV CGO_ENABLED=0

WORKDIR /src/de-mailer
COPY . .
RUN go test ./... && \
    go build .

FROM scratch

WORKDIR /app

COPY --from=0 /src/de-mailer/de-mailer /bin/de-mailer
COPY --from=0 /src/de-mailer/templates /app/templates

ENTRYPOINT ["de-mailer"]

