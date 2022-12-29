package cmd

import (
	"github.com/urfave/cli/v2"

	"bro/setting"
)

func setup(ctx *cli.Context) {
	setting.InitSetting()
	setting.Config.Debug = setting.Config.Debug || ctx.Bool("debug")
}
