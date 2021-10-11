package promote

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/apex/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	mocks "gitlab.com/unboundsoftware/apex-mocks"

	"github.com/buildtool/build-tools/pkg/version"
)

func TestDoPromote(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		descriptor  string
		args        []string
		env         map[string]string
		want        int
		wantLogged  []string
		wantCommits int
	}{
		{
			name:       "invalid argument",
			config:     "",
			args:       []string{"--unknown"},
			want:       -1,
			wantLogged: []string{"info: error: unknown flag --unknown\n"},
		},
		{
			name: "help",
			config: `ci: []
`,
			args: []string{"--help"},
			want: 0,
			wantLogged: []string{
				"info: Usage:  <target>\n",
				"info: \n",
				"info: Arguments:\n",
				"info:   <target>    the target in the .buildtools.yaml\n",
				"info: \n",
				"info: Flags:\n",
				"info:   -h, --help                Show context-sensitive help.\n",
				"info:       --version             Print args information and exit\n",
				"info:   -v, --verbose             Enable verbose mode\n",
				"info:       --config              Print parsed config and exit\n",
				"info:   -n, --namespace=STRING    override the namespace for default deployment target\n",
				"info:       --tag=STRING          override the tag to deploy, not using the CI or VCS\n",
				"info:                             evaluated value\n",
				"info:       --url=STRING          override the URL to the Git repository where files\n",
				"info:                             will be generated\n",
				"info:       --path=STRING         override the path in the Git repository where files\n",
				"info:                             will be generated\n",
				"info:       --user=\"git\"          username for Git access\n",
				"info:       --key=STRING          private key for Git access \\(defaults to\n",
				"info:                             ~\\/.ssh\\/id_rsa\\)\n",
				"info:       --password=STRING     password for private key\n",
				"info:   -o, --out=STRING          write output to specified file instead of committing\n",
				"info:                             and pushing to Git\n",
			},
		},
		{
			name: "broken config",
			config: `ci: []
`,
			args:       []string{"dummy"},
			want:       -1,
			wantLogged: []string{"error: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!seq into config.CIConfig"},
		},
		{
			name:       "missing target",
			config:     "",
			args:       []string{"dummy"},
			want:       -2,
			wantLogged: []string{"error: no gitops matching dummy found"},
		},
		{
			name: "no CI",
			config: `
gitops:
  dummy: {}
`,
			args:       []string{"dummy"},
			want:       -3,
			wantLogged: []string{"error: Commit and/or branch information is <red>missing</red>. Perhaps your not in a Git repository or forgot to set environment variables?"},
		},
		{
			name: "no deployment descriptors",
			config: `
gitops:
  dummy:
    url: "{{.repo}}"
`,
			args: []string{"dummy"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want:       -4,
			wantLogged: []string{"error: no deployment descriptors found in k8s directory"},
		},
		{
			name: "generation successful",
			config: `
git:
  name: Some User
  email: some.user@example.org
gitops:
  dummy:
    url: "{{.repo}}"
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: 0,
			wantLogged: []string{
				"info: generating...",
				"^info: pushing commit [0-9a-f]+ to .*$",
			},
			wantCommits: 1,
		},
		{
			name: "build name is normalized",
			config: `
git:
  name: Some User
  email: some.user@example.org
gitops:
  dummy:
    url: "{{.repo}}"
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy_repo",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: 0,
			wantLogged: []string{
				"info: generating...",
				"^info: pushing commit [0-9a-f]+ to .*git-repo.*\\/dummy-repo$",
			},
			wantCommits: 1,
		},
		{
			name: "other repo, path and tag",
			config: `
gitops:
  dummy:
    url: "{{.repo}}"
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy", "--url", "{{.other}}", "--path", "test/path", "--tag", "testing"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: 0,
			wantLogged: []string{
				"info: Using passed tag <green>testing</green> to promote",
				"info: generating...",
				"^info: pushing commit [0-9a-f]+ to .*other-repo.*\\/test\\/path/dummy$",
			},
			wantCommits: 0,
		},
		{
			name: "other ssh key from config",
			config: `
gitops:
  dummy:
    url: "{{.repo}}"
git:
  key: ~/other/id_rsa
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: 0,
			wantLogged: []string{
				"info: generating...",
				"^info: pushing commit [0-9a-f]+ to .*git-repo.*\\/dummy$",
			},
			wantCommits: 1,
		},
		{
			name: "clone error",
			config: `
gitops:
  dummy:
    url: "{{.repo}}"
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy", "--url", "/missing/repo"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: -4,
			wantLogged: []string{
				"info: generating...",
				"error: repository not found",
			},
			wantCommits: 0,
		},
		{
			name: "missing SSH key",
			config: `
gitops:
  dummy:
    url: "{{.repo}}"
`,
			descriptor: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  BASE_URL: https://example.org
`,
			args: []string{"dummy", "--key", "/missing/key"},
			env: map[string]string{
				"CI_COMMIT_SHA":      "abc123",
				"CI_PROJECT_NAME":    "dummy",
				"CI_COMMIT_REF_NAME": "master",
			},
			want: -4,
			wantLogged: []string{
				"info: generating...",
				"error: ssh key: open /missing/key: no such file or directory",
			},
			wantCommits: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logMock := mocks.New()
			log.SetHandler(logMock)
			oldPwd, _ := os.Getwd()
			home, _ := ioutil.TempDir(os.TempDir(), "home")
			defer func() { _ = os.RemoveAll(home) }()
			_ = os.Setenv("HOME", home)
			defer func() {
				_ = os.Unsetenv("HOME")
			}()
			sshPath := filepath.Join(home, ".ssh")
			generateSSHKey(t, sshPath)
			otherSshPath := filepath.Join(home, "other")
			generateSSHKey(t, otherSshPath)
			name, _ := ioutil.TempDir(os.TempDir(), "build-tools")
			defer func() { _ = os.RemoveAll(name) }()
			repo, _ := InitRepo(t, "git-repo")
			defer func() { _ = os.RemoveAll(repo) }()
			otherrepo, _ := InitRepo(t, "other-repo")
			defer func() { _ = os.RemoveAll(otherrepo) }()

			buff := Template(t, tt.config, repo, otherrepo)
			err := ioutil.WriteFile(filepath.Join(name, ".buildtools.yaml"), buff.Bytes(), 0777)
			assert.NoError(t, err)

			if tt.descriptor != "" {
				k8s := filepath.Join(name, "k8s")
				err = os.MkdirAll(k8s, 0777)
				assert.NoError(t, err)
				err = ioutil.WriteFile(filepath.Join(k8s, "descriptor.yaml"), []byte(tt.descriptor), 0666)
				assert.NoError(t, err)
			}
			err = os.Chdir(name)
			assert.NoError(t, err)
			defer func() { _ = os.Chdir(oldPwd) }()

			for k, v := range tt.env {
				err := os.Setenv(k, v)
				assert.NoError(t, err)
			}
			defer func() {
				for k := range tt.env {
					err := os.Unsetenv(k)
					assert.NoError(t, err)
				}
			}()
			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = Template(t, a, repo, otherrepo).String()
			}
			if got := DoPromote(name, version.Info{}, args...); got != tt.want {
				t.Errorf("DoPromote() = %v, want %v", got, tt.want)
			}
			CheckLogged(t, tt.wantLogged, logMock.Logged)

			gotCommits := CountCommits(t, repo)
			assert.Equal(t, tt.wantCommits, gotCommits)
		})
	}
}

func generateSSHKey(t *testing.T, dir string) {
	err := os.MkdirAll(dir, 0777)
	assert.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemFile, err := os.Create(filepath.Join(dir, "id_rsa"))
	assert.NoError(t, err)
	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	err = pem.Encode(pemFile, keyBlock)
	assert.NoError(t, err)
}

func CheckLogged(t testing.TB, wantLogged []string, gotLogged []string) {
	t.Helper()
	if len(gotLogged) != 0 || len(wantLogged) != 0 {
		if len(gotLogged) != len(wantLogged) {
			assert.Equal(t, wantLogged, gotLogged)
		}
		for i, got := range gotLogged {
			assert.Regexp(t, wantLogged[i], got)
		}
	}
}

func InitRepo(t *testing.T, prefix string) (string, plumbing.Hash) {
	repo, err := ioutil.TempDir(os.TempDir(), prefix)
	assert.NoError(t, err)
	gitrepo, err := git.PlainInit(repo, false)
	assert.NoError(t, err)
	tree, err := gitrepo.Worktree()
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(repo, "file"), []byte("test"), 0666)
	assert.NoError(t, err)
	_, err = tree.Add("file")
	assert.NoError(t, err)
	hash, err := tree.Commit("Test", &git.CommitOptions{Author: &object.Signature{Email: "test@example.com"}})
	assert.NoError(t, err)
	return repo, hash
}

func CountCommits(t *testing.T, repo string) int {
	gitrepo, err := git.PlainOpen(repo)
	assert.NoError(t, err)
	iter, err := gitrepo.Log(&git.LogOptions{})
	assert.NoError(t, err)
	gotCommits := -1
	for {
		_, err := iter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			assert.NoError(t, err)
		}
		gotCommits++
	}
	return gotCommits
}

func Template(t *testing.T, text, repo, otherrepo string) *bytes.Buffer {
	tpl, err := template.New("config").Parse(text)
	assert.NoError(t, err)
	buff := &bytes.Buffer{}
	err = tpl.Execute(buff, map[string]string{
		"repo":  repo,
		"other": otherrepo,
	})
	assert.NoError(t, err)
	return buff
}