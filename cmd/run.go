package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/spf13/cobra"
)

var (
	runDate        int64
	implementation string
	testcase       string
	timeout        time.Duration
)

var scenarioParameterStrings = map[string]string{
	"simple-p2p": "simple-p2p --delay=%v --bandwidth=%v --queue=%v",
}

func init() {
	runCmd.Flags().Int64VarP(&runDate, "date", "d", time.Now().Unix(), "Unix Timestamp in seconds since epoch")
	runCmd.Flags().StringVarP(&implementation, "implementation", "i", "rtq-go-scream", "implementation from implementation.json to use")
	runCmd.Flags().StringVarP(&testcase, "testcase", "c", "simple-p2p-1", "test case to run")
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

		var ts TestCases
		err = parseJSONFile("testcases.json", &ts)
		if err != nil {
			return err
		}
		t, ok := ts[testcase]
		if !ok {
			return fmt.Errorf("testcase not found: %v", testcase)
		}
		t.Name = testcase

		return run(&Config{
			Date:           time.Unix(runDate, 0),
			Implementation: i,
			TestCase:       t,
			Timeout:        timeout,
		})
	},
}

func getScenarioString(scenario Scenario) string {
	return fmt.Sprintf(
		scenarioParameterStrings[scenario.Name],
		scenario.Delay,
		scenario.Bandwidth,
		scenario.Queue,
	)
}

func run(c *Config) error {
	err := saveToJSONFile("config.json", c)
	if err != nil {
		return err
	}

	cmd := exec.Command("docker-compose", "up", "--abort-on-container-exit", "--force-recreate")

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	cmd.Env = os.Environ()
	for k, v := range map[string]string{
		"SCENARIO": getScenarioString(c.TestCase.Scenario),
		"SENDER":   c.Implementation.Sender.Image,
		"RECEIVER": c.Implementation.Receiver.Image,
		"VIDEOS":   path.Join("input", c.TestCase.VideoFile.Name),

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
