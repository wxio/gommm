package gommm

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Builder interface {
	Build() error
	Binary() string
	Errors() string
}

type builder struct {
	dir         string
	binary      string
	errors      string
	wd          string
	gomodvendor bool
	buildArgs   []string
	logger      *log.Logger
}

// NewBuilder constructor
func NewBuilder(dir string, bin string, wd string, logger *log.Logger, gomodvendor bool, buildArgs []string) Builder {
	if len(bin) == 0 {
		bin = "bin"
	}

	// does not work on Windows without the ".exe" extension
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(bin, ".exe") { // check if it already has the .exe extension
			bin += ".exe"
		}
	}

	return &builder{dir: dir, binary: bin, wd: wd, gomodvendor: gomodvendor, buildArgs: buildArgs, logger: logger}
}

func (b *builder) Binary() string {
	return b.binary
}

func (b *builder) Errors() string {
	return b.errors
}

func (b *builder) Build() error {
	if b.gomodvendor {
		gmv := exec.Command("go", "mod", "vendor")
		gmv.Dir = b.dir
		b.logger.Printf("go mod vendor\n")
		output, err := gmv.CombinedOutput()
		if err != nil {
			b.logger.Printf("go mod vendor err:%v\n%s\n", err, string(output))
		} else if !gmv.ProcessState.Success() {
			b.logger.Printf("go mod vendor no successful out:\n%s\n", string(output))
		}
	}
	args := append([]string{"go", "build", "-o", filepath.Join(b.wd, b.binary)}, b.buildArgs...)
	var command *exec.Cmd
	command = exec.Command(args[0], args[1:]...)
	command.Dir = b.dir
	output, err := command.CombinedOutput()
	if err != nil {
		b.logger.Printf("build error err:%s\ncmd:%v\nout:\n%s\n", err, args, string(output))
		b.errors = err.Error() + "\n" + string(output)
	} else if command.ProcessState.Success() {
		b.errors = ""
	} else {
		b.logger.Printf("build status error\n  cmd:%v\n  out:\n%s\n", args, string(output))
		b.errors = string(output)
	}
	if len(b.errors) > 0 {
		return fmt.Errorf(b.errors)
	}
	return err
}
