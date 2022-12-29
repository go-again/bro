package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/shlex"
	"github.com/unknwon/com"
	"github.com/unknwon/log"
	"github.com/urfave/cli/v2"
	"github.com/xlab/treeprint"
	"gopkg.in/fsnotify/fsnotify.v1"

	"bro/setting"
)

var (
	lastBuild time.Time
	eventTime = make(map[string]int64)

	runningCmd  *exec.Cmd
	runningLock = &sync.Mutex{}
	shutdown    = make(chan bool)
)

var Run = &cli.Command{
	Name:   "run",
	Usage:  "Starts watching and helping",
	Action: runCommand,
	Flags:  []cli.Flag{},
}

// isTmpFile returns true if the event was for temporary files.
func isTmpFile(name string) bool {
	if strings.HasSuffix(strings.ToLower(name), ".tmp") {
		return true
	}
	return false
}

// hasWatchExt returns true if the file name has watched extension.
func hasWatchExt(name string) bool {
	for _, ext := range setting.Config.Run.Watch.Extensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// command represents a command to run after notified.
type command struct {
	Envs []string
	Name string
	Args []string

	osEnvAdded bool
}

func (cmd *command) String() string {
	if len(cmd.Envs) > 0 {
		a := append(cmd.Envs, append([]string{cmd.Name}, cmd.Args...)...)
		return fmt.Sprint(a)
	}
	a := append([]string{cmd.Name}, cmd.Args...)
	return fmt.Sprint(a)
}

func parseCommand(cmd string) *command {
	args, _ := shlex.Split(cmd)
	runCmd := new(command)
	i := 0
	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			break
		}
		runCmd.Envs = append(runCmd.Envs, arg)
		i++
	}

	runCmd.Name = args[i]
	runCmd.Args = args[i+1:]
	return runCmd
}

func parseCommands(cmds []string) []*command {
	runCmds := make([]*command, len(cmds))
	for i, cmd := range cmds {
		runCmds[i] = parseCommand(cmd)
	}
	return runCmds
}

func envFromFiles() []string {
	envs := make([]string, 0)

	for _, envFile := range setting.Config.Run.Environment.Files {
		b, err := os.ReadFile(envFile)
		if err != nil {
			log.Warn("Failed to read environment file %q: %v", envFile, err)
			continue
		}

		envLines := strings.Split(string(b), "\n")
		for _, env := range envLines {
			envs = append(envs, strings.TrimPrefix(env, "export "))
		}
	}

	return envs
}

func notify(cmds []*command, doneChan *chan bool) {
	runningLock.Lock()
	defer func() {
		runningCmd = nil
		runningLock.Unlock()
	}()

	for _, cmd := range cmds {
		command := exec.Command(cmd.Name, cmd.Args...)

		if !cmd.osEnvAdded {
			command.Env = append(command.Env, os.Environ()...)
		}
		envFromFiles := envFromFiles()
		if len(envFromFiles) > 0 {
			command.Env = append(command.Env, envFromFiles...)
		}

		if len(setting.Config.Run.Environment.Variables) > 0 {
			command.Env = append(command.Env, setting.Config.Run.Environment.Variables...)
		}

		command.Env = append(command.Env, cmd.Envs...)

		//fmt.Println(command.Env)

		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			log.Error("Failed to start command: %v - %v", cmd, err)
			fmt.Print("\x07")
			return
		}

		log.Info("Running: %s", cmd)
		if setting.Config.Debug {
			log.Debug("Environment: %v", command.Environ())
		}
		runningCmd = command
		done := make(chan error)
		go func() {
			done <- command.Wait()
		}()

		isShutdown := false
		select {
		case err := <-done:
			if isShutdown {
				return
			} else if err != nil {
				log.Warn("Failed to execute command: %v - %v", cmd, err)
				fmt.Print("\x07")
				return
			}
		case <-shutdown:
			isShutdown = true
			gracefulKill()
			return
		}
	}
	if doneChan != nil {
		*doneChan <- true
	}
}

func gracefulKill() {
	// Directly kill the process on Windows or under request.
	if runtime.GOOS == "windows" || !setting.Config.Run.Graceful {
		runningCmd.Process.Kill()
		return
	}

	// Given process a chance to exit itself.
	runningCmd.Process.Signal(os.Interrupt)

	// Wait for timeout, and force kill after that.
	timeout := setting.Config.Run.Timeout * 1000
	for i := 0; i < timeout/100; i++ {
		time.Sleep(100 * time.Millisecond)

		if runningCmd.ProcessState == nil || runningCmd.ProcessState.Exited() {
			return
		}
	}

	log.Info("Failed to restart gracefully...")
	runningCmd.Process.Kill()
}

func catchSignals() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	<-sigs

	if runningCmd != nil {
		shutdown <- true
	}
	os.Exit(0)
}

func runCommand(ctx *cli.Context) error {
	setup(ctx)

	initCmds := parseCommands(setting.Config.Run.Init)
	runCmds := parseCommands(setting.Config.Run.Commands)
	for _, cmd := range runCmds {
		initCmds = append(initCmds, cmd)
	}

	//initDone := make(chan bool)
	go catchSignals()

	watchPaths := append([]string{setting.WorkDir}, setting.Config.Run.Watch.Dirs...)
	if setting.Config.Run.Watch.SubDirectories {
		subdirs := make([]string, 0, 10)
		for _, dir := range watchPaths[1:] {
			var dirs []string
			var err error
			if setting.Config.Run.Watch.Symlinks {
				dirs, err = com.LgetAllSubDirs(setting.UnpackPath(dir))
			} else {
				dirs, err = com.GetAllSubDirs(setting.UnpackPath(dir))
			}

			if err != nil {
				log.Fatal("Failed to get sub-directories: %v", err)
			}

			for i := range dirs {
				if !setting.IgnoreDir(dirs[i]) {
					subdirs = append(subdirs, path.Join(dir, dirs[i]))
				}
			}
		}
		watchPaths = append(watchPaths, subdirs...)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to create new watcher: %v", err)
	}
	defer watcher.Close()

	if true {
		log.Info("Watching directories in %s:", setting.WorkDir)
		root := treeprint.New()
		branches := make(map[string]*treeprint.Tree)
		for i, p := range watchPaths {
			if err = watcher.Add(setting.UnpackPath(p)); err != nil {
				log.Fatal("Failed to watch directory(%s): %v", p, err)
			}
			if i == 0 || p == "." {
				continue
			}

			if true {
				pathSliced := strings.Split(p, string(os.PathSeparator))
				current := root
				for n, pathPart := range pathSliced {
					b := strings.Join(pathSliced[0:n+1], "/")
					if branch, ok := branches[b]; ok {
						current = *branch
						continue
					} else {
						current = current.AddBranch(pathPart)
						branches[b] = &current
					}
				}
			}
		}
		fmt.Print(root.String())
	}

	go notify(initCmds, nil)

	//<-initDone

	go func() {

		for {
			select {
			case e := <-watcher.Events:
				needsNotify := true

				if isTmpFile(e.Name) || !hasWatchExt(e.Name) || setting.IgnoreFile(e.Name) {
					continue
				}

				// Prevent duplicated builds
				if lastBuild.Add(time.Duration(setting.Config.Run.Delay) * time.Millisecond).
					After(time.Now()) {
					continue
				}
				lastBuild = time.Now()

				showName := e.String()
				showName = strings.Replace(showName, setting.WorkDir, "$WORKDIR", 1)

				if e.Op&fsnotify.Remove != fsnotify.Remove && e.Op&fsnotify.Rename != fsnotify.Rename {
					mt, err := com.FileMTime(e.Name)
					if err != nil {
						log.Error("Failed to get file modified time: %v", err)
						continue
					}
					if eventTime[e.Name] == mt {
						log.Info("Skipped %s", showName)
						needsNotify = false
					}
					eventTime[e.Name] = mt
				}

				if needsNotify {
					log.Info(showName)
					if runningCmd != nil && runningCmd.Process != nil {
						if runningCmd.Args[0] == "sudo" && runtime.GOOS == "linux" {
							// Send a TERM signal to the parent process, attempting to kill it and its children
							rootCmd := exec.Command("sudo", "kill", "-TERM", com.ToStr(runningCmd.Process.Pid))
							rootCmd.Stdout = os.Stdout
							rootCmd.Stderr = os.Stderr
							if err := rootCmd.Run(); err != nil {
								log.Error("Failed to start command using sudo %s", err.Error())
								fmt.Print("\x07")
							}
						} else {
							shutdown <- true
						}
					}
					go notify(runCmds, nil)
				}
			}
		}
	}()

	select {}
	return nil
}
