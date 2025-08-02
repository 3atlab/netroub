package model

import (
	"log"
	"os"
	"path/filepath"
)

func ConfigTemplate() string {
	return `NAME:
	{{.Name}} - {{.Usage}}
	
 USAGE:
	{{.HelpName}} [scenario file] 
	
 VERSION:
	{{.Version}}
	
 AUTHOR:
	{{range $author := .Authors}}{{ $author }}{{end}}
	
 COMMANDS:
	{{range .VisibleCommands}}{{join .Names ", "}}{{ "\t" }}{{.Usage}}{{ "\n" }}{{end}}
	
 GLOBAL OPTIONS:
	{{range .VisibleFlags}}{{.}}
	{{end}}
 `
}

func SudoCheck() {
	user := os.Geteuid()
	if user != 0 {
		log.Fatal("netroub need sudo privileges to run")
	}
}

func FindTopoPath() string {
	topoPath := Scenar.Topo
	return filepath.Dir(topoPath) + "/"
}

// func FindTopoPath() string {
// 	var path string
// 	//Find the directory to search log file
// 	topoPath := Scenar.Topo
// 	nbDash := strings.Count(topoPath, "/")
// 	splittedPath := strings.Split(topoPath, "/")
// 	for i := 0; i < nbDash; i++ {
// 		path += splittedPath[i] + "/"
// 	}
//
// 	return path
// }

func StockInitialSize(initialSizes map[string]int64, root string) (map[string]int64, error) {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".log" {
			initialSizes[path] = info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return initialSizes, nil
}
