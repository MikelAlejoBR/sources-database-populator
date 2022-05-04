# sources-database-populator
This utility helps populating the database with dummy data for easier load testing.

Beware that this program may use high loads of CPU, memory and network sockets at the same time. If you find your
computer, the back end or Kubernetes struggling to keep up with the simultaneous requests, consider tweaking the
`CONCURRENT_REQUESTS` environment variable, which controls the number of active requests that this program is allowed
to send at the same time.

## Environment variables to run the program

### Required environment variables

| Environment variable | Example value      |
|:--------------------:|:------------------:|
| `SOURCES_API_HOST`   | `http://localhost` |
| `SOURCES_API_PORT`   | 8000               |

### Optional environment variables

| Environment variable           | Default value |
|:------------------------------:|:-------------:|
| `CONCURRENT_REQUESTS`          | 10            |
| `LOG_LEVEL`                    | info          |
| `NUMBER_OF_TENANTS`            | 3             |
| `SOURCES_PER_TENANT`           | 10            |
| `RHC_CONNECTIONS_PER_TENANT`   | 10            |
| `APPLICATIONS_PER_SOURCE`      | 10            |
| `ENDPOINTS_PER_SOURCE`         | 10            |
| `AUTHENTICATIONS_PER_RESOURCE` | 3             |

_**Note**: the log level can be one of "debug", "info" or "error"._
