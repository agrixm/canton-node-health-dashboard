# Canton Node Health Dashboard

Open-source monitoring agent and Prometheus/Grafana-compatible dashboard for Canton validator nodes. Covers sync lag, sequencing failures, BFT quorum drops, validator uptime, and liveness reward collection rate. Deployable by any operator in under 30 minutes with a single Docker Compose command.

## Features

*   **Sync Lag:** Monitor how far behind your node is from the latest sequenced event.
*   **Sequencing Failures:** Track the number of failed sequencing attempts.
*   **BFT Quorum Drops:** Detect when your node loses connection to enough other nodes to break the BFT quorum.
*   **Validator Uptime:** Measure the percentage of time your validator node is online and participating in the network.
*   **Liveness Reward Collection Rate:** Track the success rate of collecting liveness rewards.

## Quickstart (30 minutes)

This guide assumes you have Docker and Docker Compose installed.

1.  **Clone the repository:**

    ```bash
    git clone <repository_url>
    cd canton-node-health-dashboard
    ```

2.  **Configure the environment:**

    Create a `.env` file in the project root, based on `.env.example`, and set the following variables:

    ```
    CANTON_NODE_ADDRESS=http://<your_canton_node_ip>:<canton_admin_port>  # e.g., http://127.0.0.1:5011
    PROMETHEUS_PORT=9090  # Port for Prometheus to listen on (default: 9090)
    GRAFANA_PORT=3000     # Port for Grafana to listen on (default: 3000)
    ```

    **Important:** Replace `<your_canton_node_ip>` and `<canton_admin_port>` with the actual IP address and admin port of your Canton node. The admin port allows read-only access to operational metrics, and must be exposed by your Canton node configuration.

3.  **Start the monitoring stack:**

    ```bash
    docker-compose up -d
    ```

    This command will start the following containers:

    *   **Prometheus:** Collects metrics from the Canton node via the monitoring agent.
    *   **Grafana:** Provides a web-based dashboard for visualizing the metrics.
    *   **Canton Node Exporter:** A small agent that polls your Canton node's admin API and exports Prometheus metrics.

4.  **Access the Grafana dashboard:**

    Open your web browser and navigate to `http://localhost:3000` (or the port you configured in the `.env` file). The default Grafana username is `admin` and the default password is `admin`. You will be prompted to change the password on first login.

5.  **Import the dashboard:**

    *   In Grafana, click on the "+" icon in the left sidebar and select "Import".
    *   Upload the `grafana/dashboard.json` file from this repository.
    *   Select the Prometheus data source that was automatically configured.
    *   Click "Import".

    You should now see the Canton Node Health Dashboard with real-time metrics from your Canton node.

## Configuration

The following environment variables can be configured in the `.env` file:

*   `CANTON_NODE_ADDRESS`: The address of the Canton node's admin API (e.g., `http://127.0.0.1:5011`).
*   `PROMETHEUS_PORT`: The port for Prometheus to listen on (default: `9090`).
*   `GRAFANA_PORT`: The port for Grafana to listen on (default: `3000`).
*   `SCRAPE_INTERVAL`: How often the exporter polls metrics from the Canton node, in seconds (default: `15`). Adjust if needed based on the load this puts on your Canton node.
*   `CANTON_NODE_TIMEOUT`: Timeout for requests to the Canton node, in seconds (default: `5`).

## Architecture

The dashboard leverages the following components:

*   **Canton Node:**  The validator node you are monitoring.  It must have its admin API enabled and accessible.
*   **Canton Node Exporter:** A lightweight agent written in Python that polls the Canton node's admin API, transforms the data, and exposes it in Prometheus format.
*   **Prometheus:**  A time-series database that collects and stores the metrics exposed by the exporter.
*   **Grafana:** A dashboarding and visualization tool that displays the metrics collected by Prometheus in an easily understandable format.

## Contributing

We welcome contributions to this project!  Please see the `CONTRIBUTING.md` file for guidelines.

## License

This project is licensed under the Apache 2.0 License - see the `LICENSE` file for details.