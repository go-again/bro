# bro - runs commands when files changed

`bro` is your _bro_ who watches your back in command line.

### Installation

```bash
go get github.com/go-again/bro@latest
```

## Usage

```
bro run
```

```
USAGE:
   bro [global options] command [command options] [arguments...]

COMMANDS:
   init     Initializes config file
   run      Starts watching and helping
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d    enable debug output (default: false)
   --help, -h     show help (default: false)
   --version, -v  print the version (default: false)
```

## Quick Start

To work with a new project, you need `bro.yaml` file under the work directory. You can quickly generate by running:

```
$ bro init
```

## FAQs

### How do I gracefully restart an application?

Change following values in your `bro.yaml`:

```yaml
run:

  timeout: 5
  graceful: true
```

This will send `os.Interrupt` signal first and wait for `5` seconds before killing it forcefully.

## Configuration

An example configuration is available as [bro.yaml](templates/bro.yaml).
