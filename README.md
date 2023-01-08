# MPCPeer

[![Go test](https://github.com/dedis/cs438/actions/workflows/go_test.yml/badge.svg)](https://github.com/dedis/cs438/actions/workflows/go_test.yml)
[![Go lint](https://github.com/dedis/cs438/actions/workflows/go_lint.yml/badge.svg)](https://github.com/dedis/cs438/actions/workflows/go_lint.yml)

Project for the "Distributed System Engineering" course.

Provided by the DEDIS lab at EPFL.

## How to Start

### Quick setup

Install go >= 1.19.

Run a node:

```sh
cd gui
go run mod.go start
```

Then open the web GUI page `gui/web/index.html` and enter the peer's proxy
address provided by the peer's log: `proxy server is ready to handle requests at
'127.0.0.1:xxxx'`. You can run as many peers as wanted and connect them together
using the "routing table" > "add peer" section in the WEB GUI.

### Run the tests

See commands in the Makefile. For example: `make test`.
