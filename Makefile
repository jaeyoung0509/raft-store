.PHONY: up down build clean test cluster-status add-node remove-node put get proto 


PROTO_DIR=proto
PROTO_OUT=internal/api/pb

# Docker Compose commands
up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose build

clean:
	docker-compose down -v
	rm -rf raft-data

proto:
	protoc  \
		-I${PROTO_DIR} \
		--go_out=${PROTO_OUT} \
		--go-grpc_out=${PROTO_OUT} \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		${PROTO_DIR}/*.proto

# Cluster management commands
cluster-status:
	@echo "Checking cluster status..."
	@curl -s http://localhost:8081/cluster | jq .

add-node:
	@if [ "$(id)" = "" ] || [ "$(addr)" = "" ]; then \
		echo "Usage: make add-node id=<node-id> addr=<node-addr>"; \
		exit 1; \
	fi
	@echo "Adding node $(id) with address $(addr)..."
	@curl -X POST http://localhost:8081/cluster \
		-H "Content-Type: application/json" \
		-d "{\"id\":\"$(id)\",\"addr\":\"$(addr)\"}" 

remove-node:
	@if [ "$(id)" = "" ]; then \
		echo "Usage: make remove-node id=<node-id>"; \
		exit 1; \
	fi
	@echo "Removing node $(id)..."
	@curl -X DELETE "http://localhost:8081/cluster?id=$(id)" | jq .

# Key-value operations
put:
	@if [ "$(key)" = "" ] || [ "$(value)" = "" ]; then \
		echo "Usage: make put key=<key> value=<value>"; \
		exit 1; \
	fi
	@echo "Putting key: $(key), value: $(value)..."
	@curl -X PUT http://localhost:8081/key \
		-H "Content-Type: application/json" \
		-d "{\"key\":\"$(key)\",\"value\":\"$(value)\"}" 

get:
	@if [ "$(key)" = "" ]; then \
		echo "Usage: make get key=<key>"; \
		exit 1; \
	fi
	@echo "Getting value for key: $(key)..."
	@curl -s "http://localhost:8081/key?key=$(key)" | jq .

# Development commands
test:
	go test -v ./...
	

# Helper commands
help:
	@echo "Available commands:"
	@echo "  make up              - Start the cluster"
	@echo "  make down            - Stop the cluster"
	@echo "  make build           - Build the cluster"
	@echo "  make clean           - Clean up volumes and data"
	@echo "  make cluster-status  - Check cluster status"
	@echo "  make add-node        - Add a new node (id=<node-id> addr=<node-addr>)"
	@echo "  make remove-node     - Remove a node (id=<node-id>)"
	@echo "  make put             - Put a key-value pair (key=<key> value=<value>)"
	@echo "  make get             - Get a value by key (key=<key>)"
	@echo "  make test            - Run tests" 


