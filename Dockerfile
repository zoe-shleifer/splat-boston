FROM golang:1.22 AS build
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /server /server
COPY data/boston_mask.bin /data/boston_mask.bin || true
ENV BIND_ADDR=:8080 BOSTON_MASK_PATH=/data/boston_mask.bin
EXPOSE 8080
ENTRYPOINT ["/server"]

