FROM docker.arvancloud.ir/golang:1.22 AS builder

RUN mkdir /app
ADD . /app
WORKDIR /app

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./...

FROM alpine:latest AS production
COPY --from=builder /app .

RUN apk add -U tzdata openssh sshpass
ENV TZ=Asia/Tehran
RUN cp /usr/share/zoneinfo/Asia/Tehran /etc/localtime
CMD ["./main"]

