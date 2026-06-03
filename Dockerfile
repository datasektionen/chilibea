FROM golang:1.26.3-alpine3.22

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY src src/
COPY templates/ templates/
COPY static/ static/

RUN --mount=type=cache,target=/root/.cache/go-build/ \
    CGO_ENABLED=0 GOOS=linux go build -o /bea ./src

CMD ["/bea"]
