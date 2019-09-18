package ci

import (
	"gitlab.com/sparetimecoders/build-tools/pkg/templating"
	"gitlab.com/sparetimecoders/build-tools/pkg/vcs"
	"os"
	"path/filepath"
	"strings"
)

type CI interface {
	Name() string
	// BuildName returns the name of the current build in lowercase
	BuildName() string
	Branch() string
	BranchReplaceSlash() string
	Commit() string
	Validate(name string) error
	Scaffold(dir string, data templating.TemplateData) (*string, error)
	Badges(name string) ([]templating.Badge, error)
	SetVCS(vcs vcs.VCS)
	Configured() bool
	Configure() error
}

type CommonCI struct {
	VCS vcs.VCS
}

func (c *CommonCI) SetVCS(vcs vcs.VCS) {
	c.VCS = vcs
}

func (c *CommonCI) BuildName() string {
	dir, _ := os.Getwd()
	return strings.ToLower(filepath.Base(dir))
}
