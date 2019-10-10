package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

	envy "github.com/codegangsta/envy/lib"
	"github.com/jpillora/opts"
	"github.com/urfave/cli"

	gin "github.com/wxio/gommm/lib"
)

var (
	startTime  = time.Now()
	logger     = log.New(os.Stdout, "[gommm] ", 0)
	colorGreen = string([]byte{27, 91, 57, 55, 59, 51, 50, 59, 49, 109})
	colorRed   = string([]byte{27, 91, 57, 55, 59, 51, 49, 59, 49, 109})
	colorReset = string([]byte{27, 91, 48, 109})
	version    = "dev"
	date       = "dev"
	commit     = "dev"
)

type gommm struct {
	Bin         string   `opts:"env=GIN_BIN,short=b" help:"Name of generated binary file"`
	Path        string   `opts:"env=GIN_PATH,short=t" help:"Path to watch files"`
	Build       string   `opts:"env=GIN_BUILD,short=d" help:"Path to build files  (defaults to same value as --path)"`
	ExcludeDir  []string `opts:"env=GIN_EXCLUDE_DIR,short=x" help:"Relative directories to exclude"`
	All         bool     `opts:"env=GIN_ALL,short=a" help:"Reloads whenever any file changes"`
	BuildArgs   []string `opts:"env=GIN_BUILD_ARGS,short=r" help:"Additional go build arguments"`
	LogPrefix   string   `opts:"env=GIN_LOG_PREFIX" help:"Log prefix"`
	EnvFile     []string `opts:"env=GIN_ENV_FILE" help:"Env files to read. Later entries take precedent, Expansion applied to vars and template (default .env)"`
	GoModVendor bool     `opts:"env=GIN_GOMOD_VENDOR" help:"run 'go mod vendor' before building"`
	FailIfFirst bool     `opts:"env=GIN_FAIL_1ST" help:"fail is first build returns an error"`
	Run         run      `opts:"mode=cmd" help:"run the command"`
	Environment env      `opts:"mode=cmd" help:"output the constructed environent"`
	Version     ver      `opts:"mode=cmd" help:"print version"`
	//
	env map[string][]envvar
}

type envvar struct {
	form string
	val  string
	file string
}

type run struct {
	*gommm
}
type env struct {
	*gommm
}
type ver struct {
	*gommm
}

func main() {
	gommm := &gommm{
		Bin:       ".gommm",
		Path:      ".",
		LogPrefix: "gommm",
	}
	gommm.Run.gommm = gommm
	gommm.Environment.gommm = gommm
	gommm.Version.gommm = gommm
	op := opts.New(gommm).
		Name("gommm").
		Complete().
		UserConfigPath().
		Parse()
	if gommm.Build == "" {
		gommm.Build = gommm.Path
	}
	if len(gommm.EnvFile) == 0 {
		gommm.EnvFile = []string{".env"}
	}
	gommm.evalenv()
	logger.SetPrefix(fmt.Sprintf("[%s] ", gommm.LogPrefix))
	op.RunFatal()
	return
}

func (cfg *gommm) evalenv() {
	cfg.env = make(map[string][]envvar)
	data := struct {
		Env map[string]string
	}{
		Env: make(map[string]string),
	}
	for _, kv := range os.Environ() {
		if i := strings.Index(kv, "="); i > 0 {
			ke, va := kv[:i], kv[i+1:]
			data.Env[ke] = va
		}
	}
	for _, env := range cfg.EnvFile {
		file := env
		if !filepath.IsAbs(env) {
			file = filepath.Join(cfg.Path, env)
		}

		fr, err := os.Open(file)
		if err != nil {
			logger.Printf("error reading env %s err %v\n", file, err)
			continue
		}
		scanner := bufio.NewScanner(fr)
		for scanner.Scan() {
			line := scanner.Text()
			kv := strings.Split(line, "#")
			if i := strings.Index(kv[0], "="); i > 0 {
				ke, va := kv[0][:i], kv[0][i+1:]
				// fmt.Printf("-- k:'%s' v:'%s'\n", ke, va)
				en := cfg.env[ke]
				val := os.ExpandEnv(va)
				tpl, err := template.New("").Parse(val)
				if err != nil {
					logger.Printf("error in template parse env %s:%s err %v\n", ke, val, err)
				} else {
					buf := bytes.Buffer{}
					err = tpl.Execute(&buf, data)
					if err != nil {
						logger.Printf("error in template execute env %s:%s err %v\n", ke, val, err)
					} else {
						val = buf.String()
					}
				}
				cfg.env[ke] = append(en, envvar{form: va, val: val, file: file})
				os.Setenv(ke, val)
				data.Env[ke] = val
			}
		}
	}
}

func (cmd *run) Run() error {
	fmt.Printf("%+v\n", cmd)
	return nil
}
func (cmd *env) Run() error {
	fmt.Printf("# env \n")
	for ke, va := range cmd.gommm.env {
		fmt.Printf("%s=%s\n", ke, va[len(va)-1].val)
	}
	fmt.Printf("# --- \n")
	return nil
}
func (cmd *ver) Run() error {
	fmt.Printf("version\t%s\ncommit\t%s\ndate\t%s\n", version, commit, date)
	return nil
}

func MainAction(c *cli.Context) {

	// // Bootstrap the environment
	// envy.Bootstrap()

	// // Set the PORT env
	// os.Setenv("PORT", appPort)

	// wd, err := os.Getwd()
	// if err != nil {
	// 	logger.Fatal(err)
	// }

	// buildArgs, err := shellwords.Parse(c.GlobalString("buildArgs"))
	// if err != nil {
	// 	logger.Fatal(err)
	// }

	// buildPath := c.GlobalString("build")
	// if buildPath == "" {
	// 	buildPath = c.GlobalString("path")
	// }
	// builder := gin.NewBuilder(buildPath, c.GlobalString("bin"), c.GlobalBool("godep"), wd, buildArgs)
	// runner := gin.NewRunner(filepath.Join(wd, builder.Binary()), c.Args()...)
	// runner.SetWriter(os.Stdout)
	// proxy := gin.NewProxy(builder, runner)

	// config := &gin.Config{
	// 	Laddr:    laddr,
	// 	Port:     port,
	// 	ProxyTo:  "http://localhost:" + appPort,
	// 	KeyFile:  keyFile,
	// 	CertFile: certFile,
	// }

	// err = proxy.Run(config)
	// if err != nil {
	// 	logger.Fatal(err)
	// }

	// if laddr != "" {
	// 	logger.Printf("Listening at %s:%d\n", laddr, port)
	// } else {
	// 	logger.Printf("Listening on port %d\n", port)
	// }

	// shutdown(runner)

	// // build right now
	// build(builder, runner, logger)

	// // scan for changes
	// scanChanges(c.GlobalString("path"), c.GlobalStringSlice("excludeDir"), all, func(path string) {
	// 	runner.Kill()
	// 	build(builder, runner, logger)
	// })
}

func EnvAction(c *cli.Context) {
	logPrefix := c.GlobalString("logPrefix")
	logger.SetPrefix(fmt.Sprintf("[%s] ", logPrefix))

	// Bootstrap the environment
	env, err := envy.Bootstrap()
	if err != nil {
		logger.Fatalln(err)
	}

	for k, v := range env {
		fmt.Printf("%s: %s\n", k, v)
	}

}

func build(builder gin.Builder, runner gin.Runner, logger *log.Logger) {
	logger.Println("Building...")
	err := builder.Build()
	if err != nil {
		logger.Printf("%sBuild failed%s\n", colorRed, colorReset)
		fmt.Println(builder.Errors())
	} else {
		logger.Printf("%sBuild finished%s\n", colorGreen, colorReset)
		runner.Run()
	}

	time.Sleep(100 * time.Millisecond)
}

type scanCallback func(path string)

func scanChanges(watchPath string, excludeDirs []string, allFiles bool, cb scanCallback) {
	for {
		filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			if path == ".git" && info.IsDir() {
				return filepath.SkipDir
			}
			for _, x := range excludeDirs {
				if x == path {
					return filepath.SkipDir
				}
			}

			// ignore hidden files
			if filepath.Base(path)[0] == '.' {
				return nil
			}

			if (allFiles || filepath.Ext(path) == ".go") && info.ModTime().After(startTime) {
				cb(path)
				startTime = time.Now()
				return errors.New("done")
			}

			return nil
		})
		time.Sleep(500 * time.Millisecond)
	}
}

func shutdown(runner gin.Runner) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		log.Println("Got signal: ", s)
		err := runner.Kill()
		if err != nil {
			log.Print("Error killing: ", err)
		}
		os.Exit(1)
	}()
}
