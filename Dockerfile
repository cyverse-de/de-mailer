FROM golang:1.25

ENV CGO_ENABLED=0

WORKDIR /src/de-mailer
COPY . .
RUN make

FROM scratch

WORKDIR /app

COPY --from=0 /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=0 /src/de-mailer/de-mailer /bin/de-mailer
COPY --from=0 /src/de-mailer/templates /app/templates

ENTRYPOINT ["de-mailer"]
