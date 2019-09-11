package stack

import (
	"gitlab.com/sparetimecoders/build-tools/pkg/file"
	"gitlab.com/sparetimecoders/build-tools/pkg/templating"
	"path/filepath"
)

type Go struct{}

func (g Go) Scaffold(dir, name string, data templating.TemplateData) error {
	if content, err := templating.Execute(goMod, data); err != nil {
		return err
	} else {
		if err := file.Write(dir, "go.mod", content); err != nil {
			return err
		}
	}
	editorconfig := `
[*.go]
indent_style = tab
indent_size = 4
`
	return file.Append(filepath.Join(dir, ".editorconfig"), editorconfig)
}

func (g Go) Name() string {
	return "go"
}

var _ Stack = &Go{}

var goMod = `
module {{ .RepositoryHost -}}{{- .RepositoryPath }}

go 1.12
`