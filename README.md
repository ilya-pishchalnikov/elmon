# Elmon: PostgreSQL Monitoring Solution

[](https://golang.org/)
[](https://opensource.org/licenses/MIT)
[](https://www.docker.com/)

**Elmon** (short for **El**ephant **Mon**itoring) is a robust, containerized solution designed to collect, store, and visualize performance metrics from PostgreSQL servers. It uses a flexible configuration to define which metrics to collect and from which servers, making it a powerful tool for DevOps engineers and database administrators.

-----

## Table of Contents

  - [Features](https://www.google.com/search?q=%23features)
  - [Architecture](https://www.google.com/search?q=%23architecture)
  - [Prerequisites](https://www.google.com/search?q=%23prerequisites)
  - [Getting Started](https://www.google.com/search?q=%23getting-started)
      - [1. Clone the Repository](https://www.google.com/search?q=%231-clone-the-repository)
      - [2. Create Environment File](https://www.google.com/search?q=%232-create-environment-file)
      - [3. Configure `config.yaml`](https://www.google.com/search?q=%233-configure-configyaml)
  - [Configuration (`config.yaml`)](https://www.google.com/search?q=%23configuration-configyaml)
      - [`log`](https://www.google.com/search?q=%23log)
      - [`metrics-db`](https://www.google.com/search?q=%23metrics-db)
      - [`grafana`](https://www.google.com/search?q=%23grafana)
      - [`db-servers`](https://www.google.com/search?q=%23db-servers)
      - [`metrics`](https://www.google.com/search?q=%23metrics)
      - [`servers-metrics-map`](https://www.google.com/search?q=%23servers-metrics-map)
  - [Deployment](https://www.google.com/search?q=%23deployment)
  - [Usage](https://www.google.com/search?q=%23usage)
  - [Development](https://www.google.com/search?q=%23development)
  - [License](https://www.google.com/search?q=%23license)

-----

## Features

  - **Declarative Metric Collection**: Define complex metric collection strategies using a single YAML configuration file.
  - **Extensible**: Add new metrics easily by writing a simple SQL script or a new Go function.
  - **Configurable Intervals**: Set global and per-metric collection intervals, timeouts, and retry policies.
  - **Centralized Storage**: Uses a dedicated PostgreSQL instance to store all historical metric data, allowing for complex analysis.
  - **Pre-built Visualization**: Comes with a Grafana service for easy visualization and dashboarding.
  - **Containerized**: The entire stack is managed with Docker and Docker Compose for easy setup and deployment.

-----

## Architecture

Elmon consists of four main containerized services that work together within a shared Docker network.

1.  **Collector (`metrics-collector`)**: The core Go application. It reads the `config.yaml`, connects to target databases, executes metric collection tasks at scheduled intervals, and writes the results to the `Metrics DB`.
2.  **Metrics DB (`postgres-monitoring`)**: A PostgreSQL database that serves as the central data store for all collected metrics.
3.  **Target DB(s) (`postgres-target`)**: The PostgreSQL server(s) you want to monitor. You can define multiple target servers in the configuration.
4.  **Grafana (`grafana`)**: The visualization platform. It is pre-configured to connect to the `Metrics DB` as a data source, allowing you to build dashboards to display the collected data.

-----

## Prerequisites

Before you begin, ensure you have the following installed on your machine:

  * **Docker**
  * **Docker Compose**

-----

## Getting Started

Follow these steps to get the Elmon stack up and running.

### 1\. Clone the Repository

```bash
git clone <your-repository-url>
cd <repository-folder>
```

### 2\. Create Environment File

The system uses a `.env` file to manage all secrets. Copy the example file and fill in your desired credentials.

```bash
cp .env.example .env
```

Now, edit the `.env` file with your preferred passwords and tokens:

```dotenv
# Credentials for the Metrics DB (for storing data)
PG_METRICS_USER=metrics_user
PG_METRICS_PASSWORD=your_strong_password_here

# Credentials for the Target DB (the one being monitored)
PG_TEST_USER=test_user
PG_TEST_PASSWORD=your_strong_password_here

# Credentials for Grafana Admin User
GF_ADMIN_PASSWORD=your_grafana_admin_password

# Grafana API Token (generate one within Grafana if needed)
METRICS_GRAFANA_TOKEN=your_grafana_api_token

# Debug mode for the collector container (1 = on, 0 = off)
ELMON_DEBUG=0
```

### 3\. Configure `config.yaml`

Modify the central `config.yaml` file to define the servers and metrics you want to monitor. See the detailed [Configuration](https://www.google.com/search?q=%23configuration-configyaml) section below for more information.

-----

## Configuration (`config.yaml`)

This single file controls the entire behavior of the collector. Below is a detailed breakdown of each section.

### `log`

Defines the logging behavior for the collector application.

```yaml
log:
  level: "debug"  # Logging level: debug, info, warn, error
  format: "json"   # Log format: json or text
  file: ""         # Optional: path to a log file
```

### `metrics-db`

Connection parameters for the PostgreSQL database where collected metrics will be stored.

```yaml
metrics-db:
  environment: "metrics-collector"
  host: "postgres-monitoring" # Use the Docker Compose service name
  port: 5432
  user: "${METRICS_DB_USER}" # Injected from .env
  password: "${METRICS_DB_PASSWORD}" # Injected from .env
  dbname: "metrics"
```

### `grafana`

Configuration for the Grafana instance.

```yaml
grafana:
  url: "http://grafana:3000" # Use the Docker Compose service name
  token: "${METRICS_GRAFANA_TOKEN}" # Injected from .env
```

### `db-servers`

A list of all PostgreSQL servers that you want to monitor.

```yaml
db-servers:
  - name: "test_target_server" # A unique name for this server
    environment: "test"
    host: "postgres-target" # Use the Docker Compose service name
    port: 5432 # The internal port within the Docker network
    user: "${METRICS_TEST_DB_USER}"
    password: "${METRICS_TEST_DB_PASSWORD}"
    DbName: "application"
```

### `metrics`

The master catalog of all available metrics that can be collected.

  - **`global`**: Default settings for all metrics. These can be overridden in individual metric definitions.
  - **`metric-groups`**: A way to logically group related metrics.
  - **`metrics`**: A list of individual metrics.
      - `collection-type`: Can be `sql` (executes a script) or `go_func` (calls a built-in Go function).
      - `sql-file`: Path to the `.sql` file to execute for this metric.

<!-- end list -->

```yaml
metrics:
  version: "1.0"
  global:
    default-interval: 30s
    default-query-timeout: 15s
    default-max-retries: 0
    default-retry-delay: 1s
  metric-groups:
    - name: database_performance
      enabled: true
      metrics:
        - name: cache_hit_ratio
          value-type: float
          collection-type: sql
          sql-file: sql/script/metrics/database_perfomance/cache_hit.sql
          interval: 1m # Override global default
```

### `servers-metrics-map`

This section links the servers defined in `db-servers` to the metrics defined in `metrics`. It's here that you decide which metrics run on which server and can override collection parameters for that specific combination.

```yaml
servers-metrics-map:
  - name: "test_target_server" # Must match a name from db-servers
    metrics:
      - name: cache_hit_ratio # Must match a name from metrics
        interval: 10s        # Override the metric's default interval just for this server
        query-timeout: 5s
      - name: total_transactions
```

-----

## Deployment

All services are managed via `docker-compose`.

  - **To start all services in the background:**

    ```bash
    docker-compose up -d
    ```

  - **To view the logs of the collector:**

    ```bash
    docker-compose logs -f metrics-collector
    ```

  - **To stop and remove all containers:**

    ```bash
    docker-compose down
    ```

-----

## Usage

Once the stack is running, you can access the Grafana UI.

1.  Open your web browser and navigate to `http://localhost:3000`.
2.  Log in with the username `admin` and the password you set for `GF_ADMIN_PASSWORD` in your `.env` file.
3.  The `Metrics DB` is already configured as a data source. You can start creating new dashboards to visualize the data being collected in the `metric_value` table.

-----

## Development

The collector container includes a debug mode to facilitate development.

1.  **Enable Debug Mode**: In your `.env` file, set `ELMON_DEBUG=1`.
2.  **Start the Stack**: Run `docker-compose up -d`. The `metrics-collector` container will start and run `tail -f /dev/null`, keeping it alive without launching the application.
3.  **Access the Container**: You can get an interactive shell inside the running container.
    ```bash
    docker-compose exec metrics-collector /bin/sh
    ```
4.  **Run Manually**: From inside the container's shell, you can now manually compile or run your application for testing purposes.

-----

## License

This project is licensed under the MIT License. See the [LICENSE](https://www.google.com/search?q=LICENSE) file for details.