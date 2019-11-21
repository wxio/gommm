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

	"github.com/jpillora/opts"
	"github.com/wxio/gommm/internal/gommm"
)

var (
	version = "dev"
	date    = "dev"
	commit  = "dev"
)

type root struct {
	Bin         string   `opts:"env=GOMMM_BIN,short=b" help:"Name of generated binary file (default .gommm)"`
	Path        string   `opts:"env=GOMMM_PATH,short=t" help:"Path to watch files (default .)"`
	Build       string   `opts:"env=GOMMM_BUILD,short=d" help:"Path to build files  (defaults to --path)"`
	ExcludeDir  []string `opts:"env=GOMMM_EXCLUDE_DIR,short=x" help:"Relative directories to exclude"`
	All         bool     `opts:"env=GOMMM_ALL,short=a" help:"Reloads whenever any file changes"`
	BuildArgs   []string `opts:"env=GOMMM_BUILD_ARGS,short=r" help:"Additional go build arguments"`
	LogPrefix   string   `opts:"env=GOMMM_LOG_PREFIX" help:"Log prefix (default gommm)"`
	EnvFile     []string `opts:"env=GOMMM_ENV_FILE" help:"Env files to read. Later entries take precedent, Expansion applied to vars and template (default .env)"`
	GoModVendor bool     `opts:"env=GOMMM_GOMOD_VENDOR" help:"run 'go mod vendor' before building"`
	FailIfFirst bool     `opts:"env=GOMMM_FAIL_1ST" help:"fail is first build returns an error"`
	Run         run      `opts:"mode=cmd" help:"run the command"`
	Environment env      `opts:"mode=cmd" help:"output the constructed environent"`
	Version     ver      `opts:"mode=cmd" help:"print version"`
	//
	env        map[string][]envvar
	startTime  time.Time
	logger     *log.Logger
	colorGreen string
	colorRed   string
	colorReset string
	count      int
}

type envvar struct {
	form string
	val  string
	file string
}

type run struct {
	rt   *root
	Args []string `opts:"mode=arg" help:"command to run"`
}
type env struct {
	rt *root
}
type ver struct {
	rt *root
}

func main() {
	gm0 := &root{}
	opts.New(gm0).Name("gommm").Complete().UserConfigPath().Parse()
	if len(gm0.EnvFile) == 0 {
		gm0.EnvFile = []string{".env"}
	}
	gm0.evalenv()
	//
	gommm := &root{
		Bin:        ".gommm",
		Path:       ".",
		LogPrefix:  "gommm",
		startTime:  time.Now(),
		logger:     log.New(os.Stdout, "[gommm] ", 0),
		colorGreen: string([]byte{27, 91, 57, 55, 59, 51, 50, 59, 49, 109}),
		colorRed:   string([]byte{27, 91, 57, 55, 59, 51, 49, 59, 49, 109}),
		colorReset: string([]byte{27, 91, 48, 109}),
	}
	gommm.Run.rt = gommm
	gommm.Environment.rt = gommm
	gommm.Version.rt = gommm
	op := opts.New(gommm).Name("gommm").Complete().UserConfigPath().Parse()
	if gommm.Build == "" {
		gommm.Build = gommm.Path
	}
	gommm.logger.SetPrefix(fmt.Sprintf("[%s] ", gommm.LogPrefix))
	op.RunFatal()
	return
}

func (cfg *root) evalenv() {
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
			cfg.logger.Printf("error reading env %s err %v\n", file, err)
			continue
		}
		scanner := bufio.NewScanner(fr)
		for scanner.Scan() {
			line := scanner.Text()
			kv := strings.Split(line, "#")
			if strings.Contains(line, "\\#") {
				kv = []string{strings.ReplaceAll(line, "\\#", "#")}
			}
			if i := strings.Index(kv[0], "="); i > 0 {
				ke, va := kv[0][:i], kv[0][i+1:]
				// fmt.Printf("-- k:'%s' v:'%s'\n", ke, va)
				en := cfg.env[ke]
				val := os.ExpandEnv(va)
				tpl, err := template.New("").Parse(val)
				if err != nil {
					cfg.logger.Printf("error in template parse env %s:%s err %v\n", ke, val, err)
				} else {
					buf := bytes.Buffer{}
					err = tpl.Execute(&buf, data)
					if err != nil {
						cfg.logger.Printf("error in template execute env %s:%s err %v\n", ke, val, err)
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
	// buildArgs, err := shellwords.Parse(c.GlobalString("buildArgs"))
	// if err != nil {
	// 	logger.Fatal(err)
	// }
	wd, err := os.Getwd()
	if err != nil {
		cmd.rt.logger.Fatal(err)
	}
	builder := gommm.NewBuilder(
		cmd.rt.Path,
		cmd.rt.Bin,
		wd,
		cmd.rt.logger,
		cmd.rt.GoModVendor,
		cmd.rt.BuildArgs,
	)
	runner := gommm.NewRunner(
		filepath.Join(wd, builder.Binary()),
		cmd.rt.logger,
		cmd.Args...,
	)
	runner.SetWriter(os.Stdout)
	// shutdown handler
	shutdown(runner)
	// build right now
	cmd.rt.build(builder, runner)
	// scan for changes
	cmd.rt.scanChanges(
		cmd.rt.Path,
		cmd.rt.ExcludeDir,
		cmd.rt.All,
		func(path string) {
			runner.Kill()
			cmd.rt.build(builder, runner)
		},
	)
	return nil
}

func (cmd *env) Run() error {
	fmt.Printf("# env \n")
	for ke, va := range cmd.rt.env {
		fmt.Printf("%s=%s\n", ke, va[len(va)-1].val)
	}
	fmt.Printf("# --- \n")
	fmt.Printf("%+v\n", *cmd.rt)

	for _, kv := range os.Environ() {
		fmt.Printf("%s\n", kv)
	}

	fmt.Printf("GOMMM_PATH=%s\n", os.Getenv("GOMMM_PATH"))

	fmt.Printf("GOMMM_PATH=%s\n", cmd.rt.Path)

	return nil
}

func (cmd *ver) Run() error {
	fmt.Printf("version\t%s\ncommit\t%s\ndate\t%s\n", version, commit, date)
	return nil
}

func (cfg *root) build(builder gommm.Builder, runner gommm.Runner) {
	cfg.logger.Println("Building...")
	err := builder.Build()
	if err != nil {
		cfg.logger.Printf("%sBuild failed%s\n", cfg.colorRed, cfg.colorReset)
		fmt.Println(builder.Errors())
		if cfg.FailIfFirst && cfg.count == 0 {
			os.Exit(1)
		}
	} else {
		cfg.logger.Printf("%sBuild finished%s\n", cfg.colorGreen, cfg.colorReset)
		_, err = runner.Run()
		if err != nil && cfg.FailIfFirst && cfg.count == 0 {
			os.Exit(1)
		}
	}
	cfg.count++
	time.Sleep(100 * time.Millisecond)
}

type scanCallback func(path string)

func (cfg *root) scanChanges(watchPath string, excludeDirs []string, allFiles bool, cb scanCallback) {
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
			if (allFiles || filepath.Ext(path) == ".go") && info.ModTime().After(cfg.startTime) {
				cb(path)
				cfg.startTime = time.Now()
				return errors.New("done")
			}
			return nil
		})
		time.Sleep(500 * time.Millisecond)
	}
}

func shutdown(runner gommm.Runner) {
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
