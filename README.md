# sources-database-populator
This utility helps populating the database with dummy data for easier load testing.

Beware that this program may use high loads of CPU, memory and network sockets at the same time. If you find your
computer, the back end or Kubernetes struggling to keep up with the simultaneous requests, consider tweaking the
`CONCURRENT_REQUESTS` environment variable, which controls the number of active requests that this program is allowed
to send at the same time.

## Environment variables to run the program

### Required values
* `SOURCES_API_HOST`. Example value: `http://localhost`
* `SOURCES_API_PORT`. Example value: `8000`

### Optional values
* `CONCURRENT_REQUESTS`. Default value: `10`.
* `LOG_LEVEL`. One of `debug`, `info`, `warn` or `error`. Default value: `info`.
* `NUMBER_OF_TENANTS`. Default value: `3`
* `SOURCES_PER_TENANT`. Default value: `10`
* `RHC_CONNECTIONS_PER_TENANT`. Default value: `10`
* `APPLICATIONS_PER_SOURCE`. Default value: `10`
* `ENDPOINTS_PER_SOURCE`. Default value: `10`
* `AUTHENTICATIONS_PER_RESOURCE`. Default value: `3`
