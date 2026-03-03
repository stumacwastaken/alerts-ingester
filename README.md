# Alerts Ingestion/Syncing service

A Demo application for ingesting services from some third party alerts service.

## Quick start
This project comes with a basic makefile and docker compose setup. For a basic demo,
run `make demo` and the system should bring up the mock alerts server, create the
migrations for the database (the sqlite file is set to mount to [db/alerts.db](db/)), and start
the ingester service. You should be able to curl/browse to `http://localhost:9999/health`
to verify the service is running.

For running in a development environment, runing `make dev` will only bring
up the alerts server, migrations will need to be run manually (see below).

Caveat: Depending on your OS and how docker is feeling, sometimes networks may
fail on startup. This is an issue I've noticed with compose on a few occassions
when you develop with different profiles. `docker system prune` or `docker networks prune`
should solve the problem.

## Dependencies and tools
There are a few "external" dependencies that this project uses:
* [Golang 1.25 or greater](https://go.dev/dl/)
* [Docker Compose](https://docs.docker.com/compose)
* [sqlc](https://sqlc.dev) (for query generation)
* [go-migrate](https://github.com/golang-migrate/migrate)
* [sqlite](https://www.sqlite.org/)

Golang and Docker are self explanatory as to why they're needed, sqlc is a
developer's choice. The tool reads the schema and queries the developer creates
and generates the golang specific code to read and write the records into go
structs. This is a bit of a time saver and keeps typos away when doing row scans.
Consider it a halfway point between a fully-powered ORM, and manually writing
queries. See [sqlc.yaml](./sqlc.yaml) for configuration details.
go-migrate is a migration tool. It relies on values from `db/migrations`.

Sqlite was chosen for ease of development. While it's a solid database and can
be used in production with the right use case (see Turso for more details)
most people would rather some other tech choice. But for a demo project, sqlite
is likely "good enough".

Additionally, there are two software specific dependencies you'll find in the
[go.mod](./go.mod) file: `modernc.org/sqlite` to act as the sqlite driver (and avoiding CGO),
and `github.com/hashicorp/go-retryablehttp`, which is a relatively lightweight way
to retry http requests from a client without creating manual retries and other
logic. It's used when reaching out to the alerts service where we can retry based
off 500s, 429s, etc.

## Setup
If you want to run this project from scratch, we can do that too with some make
file commands.

1. Make sure you have golang and other tools installed
2. run `make migrate-up` to run the migrations (this will also create the database
in the `db/` directory)
3. To be safe, you can run `make gen-queries` to generate the sqlc specific code,
however the generated code _has_ been checked in.
4. Create the binary with `make build` if you intend to run this on the command line
5. run `make dev` to stand up the demo server
6. run the binary or launch through your favourite IDE.
    * Be sure to check [local.env](./local.env) for any configuration options.
7. Watch the logs, hit `localhost:9999/health`, etc.

## Endpoints

### http://localhost:9999/alerts
For fetching alerts that have been consumed. Has two additional params of
`?page=xx&limit=yy`, for basic pagination. Limit is set to 1000 records max. Going over
the page limit will return an error, and setting the limit to a negative value will
default back to the first page. It returns a basic json array and pagination data.

Other optional query params could be added, but weren't due to time constraints. i.e: order,
`enrichment_type` filters, etc. Additionally, the limit/page could be improved to the user
experience by returning the data in a useful way. Only the absolute basics were provided.


## http://localhost:9999/sync
Will attempt to trigger an immediate "sync job". If a job is already running, it
will return a `429` error indicating that a job is already running. Otherwise, it
will return a `201` accepted response.


## http://localhost:9999/health
Returns the general health of the alerts ingester service. Database connectivity,
the last successful sync, and a list of recent failed jobs (2 hour limit, developer
choice). If the db is down or if the last 10 jobs have failed, a status of `down`
should be presented. If the failureCount is greater than 0 for the last 10 jobs, the
status should be `degraded`. This is an intentional choice on my end because even
if the status of the service is back to `ok`, I'd still like to see some historical errors
if any exist within a reasonable time window. Future work could probably improve that
logic (i.e: get recent_errors as a query param for a length).

## Levers to pull:
Configuration for the service is found in the `local.env` or `demo.env` files under
`INGESTER_XXX`.

```env
INGESTER_HOST=127.0.0.1 # host for service to listen on
INGESTER_PORT=9999 # port
INGESTER_LOG_LEVEL=DEBUG # log level (slog by defualt)
INGESTER_DB_CONNECTION_STRING=db/alerts.db # db connection string
INGESTER_SYNC_INTERVAL=3m # Sync interval, follows go standard duration parsing
INGESTER_ALERTS_SERVICE_URL=http://localhost:9200/alerts # the mock alerts server we ingest from
```

Additionally, the mock server also has a couple of levers:
```env
PORT=9200 # port
HOST=0.0.0.0 # host for mock service to listen on
ERROR_RATE="0.25" # error rate.
```

Error rate does what it says, it sends an error if the service rolls the correct number.
If the error rate is `1.0`, it'll always return an error. If it's at `0` or empty, no
errors are returned. The errors returned are either a `400`, a `429` with a `retry-after` header
and a `500`. 

Additionally, the alerts server will take a "long time" to respond to a request about 10% of
the time, where we mimic a sleep. This is useful for timeouts if we were to try to set them, 
and to check for sync jobs that are "already running".

## Design decisions, etc.
We'll ignore the mock service. 

### General approach:
This project generally tries to stay as "flat" as reasonable. migrations are in the db
folder, the main.go file is in a `cmd/` sub-directory out of developer habit, etc.
Any information for http goes through the `api/` layer, which is passed into the
`alerts` service. This is where most of the business logic lies. Could all of this
had been done inside the http controllers and relied on sqlc's `data` folder (where the
database queries are)? Yes. However, I've explicitly chosen to separate those layers
due to past experiences where projects had converted (and in various cases used both of)
SOAP, JSON (over http), gRPC, and Twirp (gRPC-esque). Frankly, I don't like having to
move my logic all over the place.

As a general rule, requests start at the api level, go into the alerts service, hit the
data layer (and are run in a transaction where appropriate) or send a sync request,
and move their way back out.

The server is a standard http server with some shutdown/middleware logic. It only handles
http and not https requests due to the belief that a system like this would usually be in some
type of K8s cluster or other service with a load balancer/reverse proxy/service mesh/etc in 
front of it.

## Enriched Data
data enrichment for an alert is generated using a random IP generator function.
The enrichment type is a collection of other cybersecurity companies that may or may not
have "threat feeds" that can also be ingested. If the IP address is an internal one, 
the value of `censys` is provided.

### Sync Runner
The sync job runner is part of the `alerts` service that relies on a go ticker to periodically
reach out to the alerts-server. This is a fairly simplistic way or running a job queue, but it's
worked for me in production, and is simple to implement. A better way would be to stand up an actual
job service (i.e: with kafka or redis for a good distributed setup), but I felt that was out of
scope. 

#### small callouts
- The service itself contains a mutex for if a sync job is already running. We aren't
really communicating data across threads/routines when a job is running, but would not be needed
in a distributed environment, that said, the `/sync` logic would need to change as it relies on
the mutex being around to check if it can send a sync request.
- If a sync job has been run within a minute of the last one when the interval ticks over,
the sync job won't run. This is an attempt to be a "good citizen" of the internet. That said,
the `/sync` endpoint does not contain this check, as we may want to spam the service to generate
data. In production, a lever to handle this logic would probably be useful in both situations.


### The syncer
You'll notice in the alerts service that we expect a `Syncer` interface-compatible struct to
be passed in. This is a decision made from a past experience where we'd run multiple services to
sync from different servers, but the core logic of timing and fetching would remain the same.
This way, if we had to sync multiple third party APIs, we could write the syncer per API, and
run the same project with different configurations (ideally by an env var to make the specific
syncer) without needing to re-write the code. This is such a generic concept that we could
arguably pull the syncer into its own directory under a `pkg/` structure, but wasn't done
for convenience sake.

### Errors
Errors sent up to the http layer can leverage the [APIError](internal/api/errors.go). So long as
a custom error (such as the [AlertError](internal/alerts/alerts.go#23)) implements the StatusCode()
value, we can return an http compatible error. This somewhat ties the http layer to the internal
workings of a project, and a more robust approach would be to create a mapper between errors (or
custom error codes in the error), but I've found http codes to be more than enough most of the time.
The "health" endpoint will not display these errors. As if we can't reliably access the health endpoint,
we should be returning a 500 and alerting/paging someone.


### Observability and metrics
Normally I'd run a "metrics" package that uses prometheus or opentelemetry to scrape useful
data and send alerts where appropriate. But given the nature of this project, 
I've opted to ignore that and have left only log lines. That said, the service does contain
middleware to check for response statuses, timing, etc. per http request.


### Data(base)
The database contains two tables currently. the `alert` table and the `alert_fetch_history`
table. An alert is tied to its fetch_history so we can determine what job was used to fetch
the alert. Other tables that could exist would be the `enrichment_type` on an alert, and
`third party API` table to tie a sync job/alert to the third party api. If there was more
time, I'd probably think of more. Like a table specific to an endpoint under management that
the IP belongs to (so we could link the IP's owner to an alert), etc. 

## Testing
Testing was, like most demo projects, was ignored for the sake of time. That said,
this project has been manually tested with various scenarios:

1. Startup:
    Verify in the logs that the startup job works correctly, see the returned values
2. hit up the `/alerts` page to verify returned values:
    * attempt playing around with query params:
        *  `http://localhost:9999/alerts?limit=9000` will return an error.
        * `http://localhost:9999/alerts?page=-1` will default to the first page.
        * `http://localhost:9999/alerts?page=<<greater_than_allowed>>` will error.
3. hitting `/sync`
    * repeatedly will return a 429 error.
    * will schedule a job.
        * if scheduled within a minute of the next job, will have the ticker throw a log
        to indicate that it's skipping the job.
4. hitting `/health`
    * shows status that makes sense.
    * shows last successful sync (no successful sync will be the default zero date in golang).
    * shows recent errors, if any.

## Future considerations:
Aside from the various items mentioned above, adding some type of basic auth check between
the mock and our service was considered. It would be added into the service via an environment
variable and would make its way into the syncer its creation. where retyrablehttp works 
(i.e: demo syncer), the client could have a middleware added to it to insert an authorization
header that would be a shared API key (or what have you, oauth for production systems, etc.)

## Problems:
If there are any problems please don't hesitate to reach out.