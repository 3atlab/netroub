package model

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

func ReadJsonScenar() error {
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println("Fail to open the scenario file")
		return err
	}
	defer file.Close()

	read, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error during reading of the scenario file")
		return err
	}

	err = json.Unmarshal(read, &Scenar)
	if err != nil {
		fmt.Println("Error while decoding json data of scenario file")
		return err
	}
	sort.Sort(Scenar)
	return nil
}

func ReadYaml() error {
	file, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Println("Fail to open the scenario file")
		return err
	}
	defer file.Close()

	read, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error during reading of the scenario file")
		return err
	}

	err = yaml.Unmarshal(read, &Scenar)
	if err != nil {
		fmt.Println("Error while decoding yaml data of scenario file")
		return err
	}
	sort.Sort(Scenar)
	return nil
}

func ReadJsonData() error {
	file, err := os.Open(Scenar.Data)
	if err != nil {
		fmt.Println("Fail to open the file")
		return err
	}
	defer file.Close()

	read, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error during reading of the file")
		return err
	}

	err = json.Unmarshal(read, &Devices)
	if err != nil {
		fmt.Println("Error while decoding json data")
		return err
	}
	return nil
}
