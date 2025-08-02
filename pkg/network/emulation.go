package network

import (
	"crypto/tls"
	"crypto/x509"

	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func EmulateNetwork() error {

	cmd := exec.Command("sudo", "containerlab", "deploy", "--topo", model.Scenar.Topo)
	out, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			errMsg := string(exitError.Stderr)
			log.Fatal(errMsg)
		} else {
			log.Fatal(err.Error())
		}
	}
	fmt.Println(string(out))
	return nil
}

func DestroyNetwork() error {
	// var topoName string
	// hostName := model.Scenar.Event[0].Host
	// nbDash := strings.Count(hostName, "-")
	// splittedPath := strings.Split(hostName, "-")
	// for i := 0; i < nbDash; i++ {
	// 	if i < nbDash-1 {
	// 		topoName += splittedPath[i] + "-"
	// 	} else {
	// 		topoName += splittedPath[i]
	// 	}
	// }

	// path, err := os.Getwd()
	// if err != nil {
	// 	fmt.Println("Error while getting the working directory")
	// 	return err
	// }

	cmd := exec.Command("sudo", "containerlab", "destroy", "--topo", model.Scenar.Topo)
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("Error while destroy the emulated network")
		return err
	}
	fmt.Println(string(out))

	// cmd = exec.Command("sudo", "rm", "-rf", path+"/"+topoName)
	// out, err = cmd.Output()
	// if err != nil {
	// 	fmt.Println("Error while suppressing file")
	// 	return err
	// }
	// fmt.Println(string(out))

	return nil
}

func CreateDockerClient(c *cli.Context) error {
	tlsCfg, err := tlsConfig(c)
	if err != nil {
		return err
	}
	chaos.DockerClient, err = container.NewClient("unix:///var/run/docker.sock", tlsCfg)
	if err != nil {
		return errors.Wrap(err, "could not create Docker client")
	}
	return nil
}

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsCfg *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsCfg = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"), //nolint:gosec
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte
			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = os.ReadFile(caCertFlag)
				if err != nil {
					return nil, errors.Wrap(err, "unable to read CA certificate")
				}
			} else {
				caCert = []byte(caCertFlag)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsCfg.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate
			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, errors.Wrap(err, "unable to load client certificate")
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, errors.Wrap(err, "unable to load client certificate")
				}
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsCfg, nil
}
