package setting

import (
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/unknwon/com"
	"github.com/unknwon/log"
	"gopkg.in/yaml.v3"
)

func init() {
	log.Prefix = "[bro]"
	log.TimeFormat = "01-02 15:04:05"
}

const ConfigName = "bro.yaml"

var (
	WorkDir string
)

var Config struct {
	Debug bool `yaml:"debug"`
	Run   struct {
		Init     []string `yaml:"init"`
		Commands []string `yaml:"commands"`
		Watch    struct {
			Dirs           []string `yaml:"directories"`
			SubDirectories bool     `yaml:"subDirectories"`
			Extensions     []string `yaml:"extensions"`
			Symlinks       bool     `yaml:"symlinks"`
		} `yaml:"watch"`
		Ignore struct {
			Directories []string         `yaml:"directories"`
			Files       []string         `yaml:"files"`
			Regexps     []*regexp.Regexp `yaml:"-"`
		} `yaml:"ignore"`

		Environment struct {
			Files     []string `yaml:"files"`
			Variables []string `yaml:"variables"`
		}

		Delay    int  `yaml:"delay"`
		Timeout  int  `yaml:"timeout"`
		Graceful bool `yaml:"graceful"`
	} `yaml:"run"`
	Sync struct {
		ListenAddr string `yaml:"listenAddr,omitempty"`
		RemoteAddr string `yaml:"remoteAddr,omitempty"`
	} `yaml:"sync,omitempty"`
}

// UnpackPath replaces special path variables and returns full path.
func UnpackPath(path string) string {
	path = strings.Replace(path, "$WORKDIR", WorkDir, 1)
	path = strings.Replace(path, "$GOPATH", com.GetGOPATHs()[0], 1)
	return path
}

// IgnoreDir determines whether specified dir must be ignored.
func IgnoreDir(dir string) bool {
	for _, s := range Config.Run.Ignore.Directories {
		if strings.Contains(dir, s) {
			return true
		}
	}
	return false
}

// IgnoreFile returns true if file path matches ignore regexp.
func IgnoreFile(file string) bool {
	for i := range Config.Run.Ignore.Files {
		if Config.Run.Ignore.Regexps[i].MatchString(file) {
			return true
		}
	}
	return false
}

func InitSetting() {
	var err error
	WorkDir, err = os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current directory: %v", err)
	}

	confPath := path.Join(WorkDir, ConfigName)
	if !com.IsFile(confPath) {
		log.Fatal("%s not found in current directory", ConfigName)
	}
	if data, err := os.ReadFile(confPath); err != nil {
		log.Fatal("Failed to read %s: %v", ConfigName, err)
	} else {
		if err = yaml.Unmarshal(data, &Config); err != nil {
			log.Fatal("Failed to parse %s: %v", ConfigName, err)
		}
	}

	if Config.Run.Timeout == 0 {
		Config.Run.Timeout = 1
	}

	// Init default ignore lists.
	Config.Run.Ignore.Directories = com.AppendStr(Config.Run.Ignore.Directories, ".git")
	Config.Run.Ignore.Regexps = make([]*regexp.Regexp, len(Config.Run.Ignore.Files))
	for i, regStr := range Config.Run.Ignore.Files {
		Config.Run.Ignore.Regexps[i], err = regexp.Compile(regStr)
		if err != nil {
			log.Fatal("Invalid regexp[%s]: %v", regStr, err)
		}
	}
}
