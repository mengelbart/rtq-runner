package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var (
	runDate        int64
	implementation string
	videoFile      string
	timeout        time.Duration
)

func init() {
	runCmd.Flags().Int64VarP(&runDate, "date", "d", time.Now().Unix(), "Unix Timestamp in seconds since epoch")
	runCmd.Flags().StringVarP(&implementation, "implementation", "i", "rtq-go", "implementation from implementation.json to use")
	runCmd.Flags().StringVarP(&videoFile, "video-file", "v", "/input/sintel_trailer.mkv", "video stream file")
	runCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "max time to wait before cancelling the test run")

	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute tests",
	RunE: func(cmd *cobra.Command, args []string) error {
		var is Implementations
		err := parseJSONFile("implementations.json", &is)
		if err != nil {
			return err
		}
		i, ok := is[implementation]
		if !ok {
			return fmt.Errorf("implementation not found: %v", implementation)
		}
		i.Name = implementation
		return run(&Config{
			Date:           time.Unix(runDate, 0),
			Implementation: i,
			VideoFile:      videoFile,
			Timeout:        timeout,
		})
	},
}

func run(c *Config) error {
	err := saveToJSONFile("config.json", c)
	if err != nil {
		return err
	}

	cmd := exec.Command("docker-compose", "up", "--abort-on-container-exit")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	cmd.Env = os.Environ()
	for k, v := range map[string]string{
		"SCENARIO": "simple-p2p --delay=15ms --bandwidth=1Mbps --queue=25",
		"SENDER":   c.Implementation.Sender.Image,
		"RECEIVER": c.Implementation.Receiver.Image,
		"VIDEOS":   c.VideoFile,

		"INPUT":  "./input",
		"OUTPUT": "./output",
	} {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
	}

	err = cmd.Start()
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
