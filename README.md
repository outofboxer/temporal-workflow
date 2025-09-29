# Monthly Billing 

This is an Encore starter for a Monthly Billing. 
<describe its functionality here>

## Build from scratch

## Prerequisites 

**Install Encore:**
- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`
  
**Docker:**
1. [Install Docker](https://docker.com)
2. Start Docker

**Run setup.sh**

Setup currently supported at macOS and Linux only. 
It starts Temporal Dev Server (temporal.io) docker and registers search parameters in the Temporal Dev Server.

On Windows, you could run setup manually from command line:
```bash
docker run --rm -p 7233:7233 -p 8233:8233 temporalio/temporal:latest server start-dev --ip 0.0.0.0
temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
temporal operator search-attribute create --namespace default --name BillingPeriodNum  --type Int
temporal operator search-attribute create --namespace default --name BillStatus --type Keyword
temporal operator search-attribute create --namespace default --name BillCurrency --type Keyword
temporal operator search-attribute create --namespace default --name BillItemCount --type Int
temporal operator search-attribute create --namespace default --name BillTotal --type Int
```


## Run app

Run app using command line from the root of this repository:

```bash
encore run
```

## Using the API


### url.shorten — Shortens a URL and adds it to the database

```bash
curl 'http://localhost:4000/url' -d '{"URL":"https://news.ycombinator.com"}'
```

### url.get — Gets a URL from the database using a short ID

```bash
curl 'http://127.0.0.1:4000/url/:id'
```

## Open the developer dashboard

While `encore run` is running, open [http://localhost:9400/](http://localhost:9400/) to access Encore's [local developer dashboard](https://encore.dev/docs/go/observability/dev-dash).

Here you can see traces for all your requests, view your architecture diagram, and see API docs in the Service Catalog.

## Connecting to databases

You can connect to your databases via psql shell:

```bash
encore db shell <database-name> --env=local --superuser
```

Learn more in the [CLI docs](https://encore.dev/docs/go/cli/cli-reference#database-management).

## Deployment

### Self-hosting

See the [self-hosting instructions](https://encore.dev/docs/go/self-host/docker-build) for how to use `encore build docker` to create a Docker image and configure it.

## Testing

```bash
encore test ./...
```


This should be done at Temporal:
-- 




docker exec -it temporalite-temporalite-1 /bin/sh