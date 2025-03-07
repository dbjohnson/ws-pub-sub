# ws-pub-sub

[![Go](https://github.com/dbjohnson/ws-pub-sub/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/dbjohnson/ws-pub-sub/actions/workflows/build-and-test.yml)

A minimal Go WebSocket-based pub/sub server with topic-based message routing.

## Features

- WebSocket-based publisher/subscriber pattern
- Topic-based message routing
- Real-time statistics via HTTP endpoint
- Simple web UI for testing
- Concurrent handling of multiple connections
- Message rate tracking (1-minute sliding window)
- Configuration via environment variables

## Requirements

- Go 1.15+

## Installation

Clone the repository:

```bash
git clone https://github.com/username/ws-pub-sub.git
cd ws-pub-sub
```

Install dependencies:

```bash
make deps
```

Or manually:

```bash
go get -u github.com/gorilla/websocket
```

## Configuration

The server can be configured using the following environment variables:

| Environment Variable | Description                          | Default Value       |
| -------------------- | ------------------------------------ | ------------------- |
| WS_HOST              | Host to bind the server to           | "" (all interfaces) |
| WS_PORT              | Port to listen on                    | "8080"              |
| WS_READ_BUFFER_SIZE  | WebSocket read buffer size in bytes  | 1024                |
| WS_WRITE_BUFFER_SIZE | WebSocket write buffer size in bytes | 1024                |

## Usage

### Building and Running

Build the server:

```bash
make build
```

Run the server with default configuration:

```bash
make run
```

Run the server with custom configuration:

```bash
WS_PORT=9090 WS_READ_BUFFER_SIZE=2048 ./ws-pub-sub
```

The server will start on http://localhost:8080 by default (or the configured port).

### Available Endpoints
- `/` - WebSocket endpoint for pub/sub operations
- `/dashboard` - Web UI for testing
- `/stats` - HTTP endpoint for server statistics

## WebSocket Protocol

The WebSocket communication uses JSON messages with the following format:

### Subscribe to a Topic

```json
{
  "action": "subscribe",
  "topic": "topic-name"
}
```

### Unsubscribe from a Topic

```json
{
  "action": "unsubscribe",
  "topic": "topic-name"
}
```

### Publish a Message

```json
{
  "action": "publish",
  "topic": "topic-name",
  "payload": {
    "your": "data",
    "goes": "here"
  }
}
```

### Receiving Messages

When subscribed to a topic, messages will be delivered in the following format:

```json
{
  "topic": "topic-name",
  "payload": {
    "your": "data",
    "goes": "here"
  }
}
```

## Statistics

The `/stats` endpoint returns server statistics in JSON format:

```json
{
  "topics": 3,
  "subscribers": 10,
  "message_rate_total": 25.5,
  "message_rate_by_topic": {
    "topic1": 10.2,
    "topic2": 8.3,
    "topic3": 7.0
  }
}
```

## Testing

Run the test suite:

```bash
make test
```

## Web UI

A simple web UI is available at the root URL (`/`) for testing the pub/sub functionality. The UI allows you to:

- Subscribe to topics
- Unsubscribe from topics
- Publish messages to topics
- View received messages
- View server statistics

## Performance Considerations

- The server is designed to handle multiple concurrent connections
- For production use, consider implementing authentication and authorization
- Message payloads are limited to 512KB by default
- WebSocket connections have a 60-second read deadline

## License

This project is licensed under the MIT License - see the LICENSE file for details.
