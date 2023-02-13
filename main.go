package main

import (
	_ "embed"
	"os"

	"github.com/unknwon/log"
	"github.com/urfave/cli/v2"

	"github.com/go-again/bro/cmd"
)

var version = "2023.02.1"

var (

	//go:embed templates/bro.yaml
	template []byte
)

func init() {
	cmd.Template = template
}

func main() {
	app := &cli.App{
		Name:  "bro",
		Usage: "runs commands when files changed",
		Commands: []*cli.Command{
			cmd.Init,
			cmd.Run,
			//cmd.Sync,
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "enable debug output",
			},
		},
		Version: version,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal("%v", err)
	}
}
