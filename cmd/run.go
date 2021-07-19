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
	runDate   int64
	sender    string
	receiver  string
	videoFile string
	timeout   time.Duration
)

func init() {
	runCmd.Flags().Int64VarP(&runDate, "date", "d", time.Now().Unix(), "Unix Timestamp in seconds since epoch")
	runCmd.Flags().StringVarP(&sender, "sender", "s", "engelbart/rtq-go-endpoint:main", "sender implementation")
	runCmd.Flags().StringVarP(&receiver, "receiver", "r", "engelbart/rtq-go-endpoint:main", "receiver implementation")
	runCmd.Flags().StringVarP(&videoFile, "video-file", "v", "/input/sintel_trailer.mkv", "video stream file")
	runCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "max time to wait before cancelling the test run")

	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute tests",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(&Config{
			Date: time.Unix(runDate, 0),
			Sender: &Implementation{
				Name: sender,
				URL:  sender,
			},
			Receiver: &Implementation{
				Name: receiver,
				URL:  receiver,
			},
			VideoFile: videoFile,
			Timeout:   timeout,
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
		"SCENARIO": "simple-p2p --delay=15ms --bandwidth=10Mbps --queue=25",
		"SENDER":   c.Sender.Name,
		"RECEIVER": c.Receiver.Name,
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
