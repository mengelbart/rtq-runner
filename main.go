package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	configs := []config{
		{
			sender:                "engelbart/rtq-go-endpoint:main",
			receiver:              "engelbart/rtq-go-endpoint:main",
			qrtCongestionControl:  "scream",
			quicCongestionControl: "cubic",
			streams:               []string{"/input/sintel_trailer.mkv"},
		},
	}

	for _, c := range configs {
		err := c.run()
		if err != nil {
			panic(err)
		}
	}
}

type config struct {
	sender                string
	receiver              string
	qrtCongestionControl  string
	quicCongestionControl string
	streams               []string
}

const timeout = 5 * time.Minute

func (c *config) run() error {
	cmd := exec.Command("docker-compose", "up", "--abort-on-container-exit")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	cmd.Env = os.Environ()
	for k, v := range map[string]string{

		"SCENARIO":                "simple-p2p --delay=15ms --bandwidth=10Mbps --queue=25",
		"SENDER":                  c.sender,
		"RECEIVER":                c.receiver,
		"QRT_CONGESTION_CONTROL":  c.qrtCongestionControl,
		"QUIC_CONGESTION_CONTROL": c.quicCongestionControl,

		"INPUT":  "./input",
		"OUTPUT": "./output",
		"VIDEOS": strings.Join(c.streams, " "),
	} {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
	}

	err := cmd.Start()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(timeout):
		err := cmd.Process.Kill()
		if err != nil {
			return err
		}
		return errors.New("process killed after timeout")
	}
	return nil
}
