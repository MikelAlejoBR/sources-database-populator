FROM registry.access.redhat.com/ubi8/ubi:latest as build
WORKDIR /build

RUN dnf -y --disableplugin=subscription-manager install go

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o sources-database-populator . && strip sources-database-populator

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=build /build/sources-database-populator /sources-database-populator
ENTRYPOINT ["/sources-database-populator"]
