package gin

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Builder interface {
	Build(logger *log.Logger) error
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
}

func NewBuilder(dir string, bin string, wd string, gomodvendor bool, buildArgs []string) Builder {
	if len(bin) == 0 {
		bin = "bin"
	}

	// does not work on Windows without the ".exe" extension
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(bin, ".exe") { // check if it already has the .exe extension
			bin += ".exe"
		}
	}

	return &builder{dir: dir, binary: bin, wd: wd, gomodvendor: gomodvendor, buildArgs: buildArgs}
}

func (b *builder) Binary() string {
	return b.binary
}

func (b *builder) Errors() string {
	return b.errors
}

func (b *builder) Build(logger *log.Logger) error {
	if b.gomodvendor {
		gmv := exec.Command("go", "mod", "vendor")
		gmv.Dir = b.dir
		logger.Printf("go mod vendor\n")
		output, err := gmv.CombinedOutput()
		if err != nil {
			logger.Printf("go mod vendor err:%v\n%s\n", err, string(output))
		} else if !gmv.ProcessState.Success() {
			logger.Printf("go mod vendor no successful out:\n%s\n", string(output))
		}
	}
	args := append([]string{"go", "build", "-o", filepath.Join(b.wd, b.binary)}, b.buildArgs...)
	var command *exec.Cmd
	command = exec.Command(args[0], args[1:]...)
	command.Dir = b.dir
	output, err := command.CombinedOutput()
	if err != nil {
		b.errors = err.Error()
	} else if command.ProcessState.Success() {
		b.errors = ""
	} else {
		b.errors = string(output) + " " + err.Error()
	}
	if len(b.errors) > 0 {
		return fmt.Errorf(b.errors)
	}
	return err
}
