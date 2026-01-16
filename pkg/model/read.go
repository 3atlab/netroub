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

// ReadScenarioFromPath reads a scenario from a file path and returns it.
// Supports both JSON and YAML formats based on file extension.
func ReadScenarioFromPath(path string) (*Scenario, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open scenario file: %w", err)
	}
	defer file.Close()

	read, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	var scenario Scenario

	// Detect format by extension
	if len(path) > 5 && path[len(path)-5:] == ".yaml" {
		err = yaml.Unmarshal(read, &scenario)
	} else if len(path) > 4 && path[len(path)-4:] == ".yml" {
		err = yaml.Unmarshal(read, &scenario)
	} else {
		err = json.Unmarshal(read, &scenario)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse scenario file: %w", err)
	}

	return &scenario, nil
}

// ReadDataFromPath reads device data from a file path and returns it.
func ReadDataFromPath(path string) (*Data, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer file.Close()

	read, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}

	var data Data
	err = json.Unmarshal(read, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data file: %w", err)
	}

	return &data, nil
}

// GetLabNameFromScenario extracts the lab name from a scenario file.
// It reads the scenario, then reads the data file to get the topology name.
// The baseDir is used to resolve relative paths in the scenario file.
func GetLabNameFromScenario(scenarioPath string, baseDir string) (string, error) {
	scenario, err := ReadScenarioFromPath(scenarioPath)
	if err != nil {
		return "", err
	}

	// Resolve data file path (may be relative to scenario file)
	dataPath := scenario.Data
	if dataPath != "" && dataPath[0] != '/' {
		dataPath = baseDir + "/" + dataPath
	}

	if dataPath == "" {
		// Fall back to scenario name if no data file
		return scenario.ScenarioName, nil
	}

	data, err := ReadDataFromPath(dataPath)
	if err != nil {
		// Fall back to scenario name if data file cannot be read
		return scenario.ScenarioName, nil
	}

	if data.Name != "" {
		return data.Name, nil
	}

	return scenario.ScenarioName, nil
}
