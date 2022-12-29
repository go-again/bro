package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/unknwon/com"
	"github.com/unknwon/log"
	"github.com/urfave/cli/v2"

	"github.com/go-again/bro/setting"
)

var Template []byte

var Init = &cli.Command{
	Name:   "init",
	Usage:  "Initializes config file",
	Action: initCommand,
	Flags:  []cli.Flag{},
}

func initCommand(ctx *cli.Context) error {
	if com.IsExist(setting.ConfigName) {
		fmt.Printf("There is %s in current directory, would you like to overwrite? (y/n): ", setting.ConfigName)
		var answer string
		fmt.Scan(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println("Not overwriting...")
			return nil
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current directory: %v", err)
	}

	data := Template

	project := filepath.Base(wd)
	if runtime.GOOS == "windows" {
		project += ".exe"
	}

	data = bytes.Replace(data, []byte("$PROJECT"), []byte(project), -1)
	if err := os.WriteFile(setting.ConfigName, data, os.ModePerm); err != nil {
		log.Fatal("Failed to generate default %s: %v", setting.ConfigName, err)
	}
	return nil
}
