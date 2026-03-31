FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk --no-cache add git

COPY go.mod ./
RUN go mod download

COPY . .

RUN rm -f go.sum && go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /finset ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /finset /app/finset
COPY --from=builder /app/derekter_qory_practice_v3\ \(1\).html /app/quiz.html

ENV FRONTEND_HTML_PATH=/app/quiz.html

EXPOSE 8080

CMD ["/app/finset"]
