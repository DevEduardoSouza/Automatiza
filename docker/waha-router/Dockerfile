FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod .
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -o /router main.go

FROM alpine:3.19
RUN adduser -D -u 10001 router
USER router
COPY --from=build /router /router
EXPOSE 3000
ENTRYPOINT ["/router"]
