package gpio

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type GPIOList struct {
	Gpio []GPIO `yaml:"gpio"`
}

func (gpio *GPIOList) Parse(fileName string, verbose bool) error {

	if verbose {
		log.Println(`Parser default options:
	Name: "",
	Chip: "",
	Line: -1
	`)
	}

	yamlFile, err := os.ReadFile(fileName)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, &gpio)
	if err != nil {
		log.Printf("Cannot unmarshal YAML file. Error: %s", err)
		return err
	}

	return nil
}
