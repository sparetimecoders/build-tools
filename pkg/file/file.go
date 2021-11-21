package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/apex/log"
)

func FindFilesForTarget(dir, target string) ([]os.FileInfo, error) {
	return filesForTarget(dir, target, "file", ".yaml")
}

func FindScriptsForTarget(dir, target string) ([]os.FileInfo, error) {
	return filesForTarget(dir, target, "script", ".sh")
}

func filesForTarget(dir, target, filetype, suffix string) ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	matching := map[string]os.FileInfo{}

	for _, info := range files {
		if strings.HasSuffix(info.Name(), suffix) {
			log.Debugf("considering %s '<yellow>%s</yellow>' for target: <green>%s</green>\n", filetype, info.Name(), target)
			if strings.HasSuffix(info.Name(), fmt.Sprintf("-%s%s", target, suffix)) || !strings.Contains(info.Name(), "-") {
				matching[info.Name()] = info
			} else {
				log.Debugf("not using %s '<red>%s</red>' for target: <green>%s</green>\n", filetype, info.Name(), target)
			}
		}
	}
	var result []os.FileInfo
	for k, v := range matching {
		if strings.HasSuffix(k, fmt.Sprintf("-%s%s", target, suffix)) {
			log.Debugf("using %s '<green>%s</green>' for target: <green>%s</green>\n", filetype, k, target)
			result = append(result, v)
		} else {
			name := fmt.Sprintf("%s-%s%s", strings.TrimSuffix(k, suffix), target, suffix)
			if _, exists := matching[name]; !exists {
				log.Debugf("using %s '<green>%s</green>' for target: <green>%s</green>\n", filetype, k, target)
				result = append(result, v)
			} else {
				log.Debugf("not using %s '<red>%s</red>' for target: <green>%s</green>\n", filetype, k, target)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result, nil
}