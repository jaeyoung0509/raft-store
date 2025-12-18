# go-store

`go-store` is a simple distributed key-value store built with Go and the Raft consensus algorithm, using the [hashicorp/raft](https://github.com/hashicorp/raft) library.

## ✨ Features

* **Raft-based consensus:** Ensures data consistency and high availability even during node failures.
* **Key-Value storage:** Supports storing simple string keys and values (internally as byte slices).
    * The Raft FSM uses in-memory storage.
    * Raft logs and snapshots are persisted using BoltDB.
* **gRPC API:** Provides gRPC endpoints for cluster management and data operations:
    * Add peer (`AddPeer`)
    * Remove peer (`RemovePeer`)
    * Get cluster status (`GetStatus`, `GetCluster`)
    * Apply data commands (`ApplyCommand`)
* **Docker-based deployment & testing:** Easily run and test a local Raft cluster with `docker-compose`.

## 🚀 Getting Started

### Prerequisites

* [Go](https://golang.org/dl/) (v1.24 or later recommended)
* [Docker](https://docs.docker.com/get-docker/)
* [Docker Compose](https://docs.docker.com/compose/install/)

### Run Locally

1. **Clone the repository:**
    ```bash
    git clone https://github.com/<your-username>/go-store.git
    cd go-store
    ```

2. **(Optional) Create a `.env` file:**
    You can define environment variables in a `.env` file in the root directory. Example: `RAFT_DATA_DIR=/tmp/my-raft-data`. Defaults are provided in `docker-compose.yml`.

3. **Start the cluster with Docker Compose:**
    ```bash
    docker-compose up --build -d
    ```
    This spins up a 3-node Raft cluster with `node1` bootstrapped as the leader.

4. **Interact with the gRPC API:**
    Use a tool like `grpcurl` to communicate with the cluster via gRPC ports (`34001`, `34002`, `34003`).

    * **Check leader status (e.g., via node1):**
        ```bash
        grpcurl -plaintext localhost:34001 api.RaftService/GetStatus
        ```

    * **Store data using `ApplyCommand`:**
        First, identify the leader node and call:
        ```bash
        grpcurl -plaintext -d '{"type": "SET", "key": "mykey", "value": "myvalue"}' localhost:34001 api.RaftService/ApplyCommand
        ```
        _Note: Reads are available via HTTP `GET /key?key=...` or gRPC `GetValue`._

5. **Stop the cluster and clean data:**
    ```bash
    docker-compose down -v
    ```

## ✅ Running Tests

To run unit tests:
```bash
go test ./...
```

