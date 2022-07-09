package conf

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
)

func confRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(home, ".kubectl-dev")
}

func Load(fileName string, yamlContent any) error {
	bytes, err := ioutil.ReadFile(filepath.Join(confRoot(), fileName))
	if err != nil {
		return err
	}

	return yaml.Unmarshal(bytes, yamlContent)
}

func Save(fileName string, yamlContent interface{}) error {
	err := os.MkdirAll(confRoot(), 0755)
	if err != nil {
		return err
	}

	bytes, err := yaml.Marshal(yamlContent)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(confRoot(), fileName), bytes, 0644)
}
