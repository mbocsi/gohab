# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

<!-- ## Development Guidelines
1. Initial Analysis and Planning
First think through the problem, read the codebase for relevant files, and write a plan to tasks/todo.md.
2. Todo List Structure
The plan should have a list of todo items that you can check off as you complete them.
3. Plan Verification
Before you begin working, check in with me and I will verify the plan.
4. Task Execution
Then, begin working on the todo items, marking them as complete as you go.
5. Communication
Please every step of the way just give me a high level explanation of what changes you made.
6. Simplicity Principle
Make every task and code change you do as simple as possible. Use existing structures and abstractions whenever it makes sense. 
We want to avoid making any massive or complex changes. Every change should impact as little code as possible. Everything is about simplicity.
7. Process Documentation
Every time you perform actions related to the project, write a log of your actions to docs/activity.md and read that file whenever you find it necessary to assist you.
8. Review Process
Finally, add a review section to the tasks/todo.md file with a summary of the changes you made and any other relevant information. -->

## Build Commands

- `make all` - Build all components (server and all client examples)
- `make server` - Build server only (outputs to `bin/server`)
- `make client1` - Build client1 example (outputs to `bin/client1`)
- `make client2` - Build client2 example (outputs to `bin/client2`)
- `make client3` - Build client3 example (outputs to `bin/client3`)
- `make clean` - Remove all built binaries from bin/ directory

## Running the Application

The server runs on port 8080 (web interface) and 8888 (TCP transport):
- `./bin/server` - Start the GoHab server
- `./bin/client1` - Run client1 example
- `./bin/client2` - Run client2 example
- `./bin/client3` - Run client3 example

## Architecture Overview

GoHab is a home automation server implementing a message-based architecture with device capabilities and pub/sub messaging.

### Core Components

**Server Architecture:**
- `GohabServer` - Main server coordinating all components
- `Broker` - Pub/sub message broker for topic-based communication
- `DeviceRegistry` - Manages connected devices and their metadata
- `Transport` - Abstraction for client connection methods (TCP, potentially others)

**Client Architecture:**
- `Client` - Main client implementation with capability management
- `Transport` - Client-side transport abstraction (TCP implementation)
- Device capabilities define what actions/data each client supports

**Message Protocol:**
- JSON-based messages with types: `identify`, `command`, `query`, `response`, `data`, `status`, `subscribe`
- Topic-based routing (e.g., "temperature", "led/brightness")
- Capability-driven: clients declare what they can do during identification

### Key Concepts

**Capabilities**: Devices declare capabilities (e.g., "temperature sensor", "LED control") with schemas defining supported methods:
- `data` - Publish sensor readings or state
- `status` - Report device status
- `command` - Accept control commands
- `query` - Respond to information requests

**Message Flow:**
1. Client connects and sends `identify` message with capabilities
2. Server assigns ID and registers device
3. Client subscribes to relevant topics
4. Bidirectional message exchange via pub/sub broker

**Web Interface**: HTML templates with HTMX for dynamic updates, showing:
- Connected devices and their capabilities
- Available features (topics) and their sources
- Transport connections and status
- Real-time updates via Server-Sent Events

## Project Structure

- `cmd/` - Entry points for server and client examples
- `server/` - Server-side components (coordinator, broker, web handlers, transports)
- `services/` - Server-side services
- `web/` - An HTMX Web interface for server management
- `client/` - Client library for connecting devices
- `proto/` - Message and capability definitions
- `templates/` - HTML go templates for web interface using HTMX for server-driven interactions
- `assets/` - Static web assets (CSS)

## Development Notes

- Uses `slog` for structured JSON logging
- Web interface uses Chi router and HTMX Go templates
- Real-time updates via Server-Sent Events for topic streaming
- MCP (Model Context Protocol) integration available but optional
- TCP transport is primary connection method, architecture supports multiple transports