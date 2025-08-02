package model

const EventTypeDummy = "dummy"
const EventTypePumba = "pumba"
const EventTypeShell = "shell"
const EventTypeConfig = "config"

type CommandOptions struct {
	Duration       string  `json:"duration" yaml:"duration"`
	Interface      string  `json:"interface" yaml:"interface"`
	Time           int     `json:"time" yaml:"time"`
	Jitter         int     `json:"jitter" yaml:"jitter"`
	Correlation    float64 `json:"correlation" yaml:"correlation"`
	Percent        float64 `json:"percent" yaml:"percent"`
	Distribution   string  `json:"distribution" yaml:"distribution"`
	Limit          int     `json:"limit" yaml:"yaml"`
	Rate           string  `json:"rate" yaml:"rate"`
	PacketOverhead int     `json:"packetOverhead" yaml:"packetOverhead"`
	CellSize       int     `json:"cellSize" yaml:"cellSize"`
	CellOverhead   int     `json:"cellOverhead" yaml:"cellOverhead"`
	StressImage    string  `json:"stressImage" yaml:"stressImage"`
	PullImage      bool    `json:"pullImage" yaml:"pullImage"`
	Stressors      string  `json:"stressors" yaml:"stressors"`
}

type ConfigFileChanges struct {
	File    string `json:"file" yaml:"file"`
	Line    int    `json:"line" yaml:"line"`
	Command string `json:"command" yaml:"command"`
}

type PumbaCommand struct {
	Name    string         `json:"name" yaml:"name"`
	Options CommandOptions `json:"options" yaml:"options"`
}

type Event struct {
	BeginTime         string              `json:"beginTime" yaml:"beginTime"`
	Type              string              `json:"type" yaml:"type"`
	Host              string              `json:"host" yaml:"host"`
	Hosts             []string            `json:"hosts" yaml:"hosts"`
	PumbaCommand      PumbaCommand        `json:"pumbaCommand" yaml:"pumbaCommand"`
	ShellPath         string              `json:"shellPath" yaml:"shellPath"`
	ShellCommands     []string            `json:"shellCommands" yaml:"shellCommands"`
	VtyshChanges      []string            `json:"vtyshChanges" yaml:"vtyshChanges"`
	ConfigFileChanges []ConfigFileChanges `json:"configFileChanges" yaml:"configFileChanges"`
}

func (e Event) GetHosts() (hosts []string) {
	if e.Host != "" {
		hosts = append(hosts, e.Host)
	}
	if len(e.Hosts) > 0 {
		hosts = append(hosts, e.Hosts...)
	}
	return hosts
}

type Scenario struct {
	ScenarioName string `json:"scenarioName" yaml:"scenarioName"`
	Topo         string `json:"topo" yaml:"topo"`
	Data         string `json:"data" yaml:"data"`
	LogPath      string `json:"logPath" yaml:"logPath"`
	// Duration is the period for data collection in the scenario
	Duration string `json:"duration" yaml:"duration"`
	// Hosts is the list of hostnames (in containerlab definition) for measurement in the scenario
	Hosts []string `json:"hosts" yaml:"hosts"`
	// LogFiles []string `json:"logfiles" yaml:"logfiles"`
	Event []Event `json:"event" yaml:"event"`
}

var Scenar Scenario

func (s Scenario) Len() int           { return len(s.Event) }
func (s Scenario) Less(i, j int) bool { return s.Event[i].BeginTime < s.Event[j].BeginTime }
func (s Scenario) Swap(i, j int)      { s.Event[i], s.Event[j] = s.Event[j], s.Event[i] }
