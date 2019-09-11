package config

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGit_Identify(t *testing.T) {
	dir, _ := ioutil.TempDir("", "build-tools")
	defer os.RemoveAll(dir)

	hash, _ := InitRepoWithCommit(dir)

	out := &bytes.Buffer{}
	result := Identify(dir, out)
	assert.NotNil(t, result)
	assert.Equal(t, "git", result.Name())
	assert.Equal(t, hash.String(), result.Commit())
	assert.Equal(t, "master", result.Branch())
	assert.Equal(t, "", out.String())
}

func TestGit_MissingRepo(t *testing.T) {
	dir, _ := ioutil.TempDir("", "build-tools")
	defer os.RemoveAll(dir)

	_ = os.Mkdir(filepath.Join(dir, ".git"), 0777)

	out := &bytes.Buffer{}
	result := Identify(dir, out)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.Commit())
	assert.Equal(t, "", result.Branch())
	assert.Equal(t, "Unable to open repository: repository does not exist\n", out.String())
}

func TestGit_NoCommit(t *testing.T) {
	dir, _ := ioutil.TempDir("", "build-tools")
	defer os.RemoveAll(dir)

	InitRepo(dir)

	out := &bytes.Buffer{}
	result := Identify(dir, out)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.Commit())
	assert.Equal(t, "", result.Branch())
	assert.Equal(t, "Unable to fetch head: reference not found\n", out.String())
}