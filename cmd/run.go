package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

var (
	runDate        int64
	implementation string
	testcase       string
	timeout        time.Duration
)

func init() {
	runCmd.Flags().Int64VarP(&runDate, "date", "d", time.Now().Unix(), "Unix Timestamp in seconds since epoch")
	runCmd.Flags().StringVarP(&implementation, "implementation", "i", "rtq-go-scream", "implementation from implementation.json to use")
	runCmd.Flags().StringVarP(&testcase, "testcase", "c", "simple-p2p-1", "test case to run")
	runCmd.Flags().DurationVarP(&timeout, "timeout", "t", 10*time.Minute, "max time to wait before cancelling the test run")

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
		"SENDER":   c.Implementation.Sender.Image,
		"RECEIVER": c.Implementation.Receiver.Image,
		"VIDEOS":   path.Join("input", c.TestCase.VideoFile.Name),

		"SENDER_PARAMS":   c.Implementation.Sender.Params,
		"RECEIVER_PARAMS": c.Implementation.Receiver.Params,
		"INPUT":           "./input",
		"OUTPUT":          "./output",
	} {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tc := trafficController{
		phases: c.TestCase.Phases,
	}

	err = cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			log.Printf("failed to get docker client: %v\n", err)
			return
		}

		containerRunning := map[string]bool{
			"/sender":   false,
			"/receiver": false,
		}
		waitingFor := len(containerRunning)
		done := false
		start := time.Now()
		for !done {
			containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
			if err != nil {
				log.Printf("failed to get container list: %v\n", err)
			}
			for _, container := range containers {
				for _, name := range container.Names {
					if running, ok := containerRunning[name]; ok && !running {
						if container.State == "running" {
							containerRunning[name] = true
							waitingFor--
							fmt.Printf("found container %v in state %v\n", name, container.State)
						}
					}
				}
			}
			if waitingFor == 0 {
				done = true
			}
			if time.Since(start) > 5*time.Minute {
				fmt.Printf("stopped waiting for container after 1 minute, skipping traffic control")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}

		log.Printf("start traffic controller after %v\n", time.Since(start))
		err = tc.run(ctx)
		if err != nil {
			log.Printf("tc controller failed: %v\n", err)
		}
	}()

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

type tcConfig struct {
	Delay   Duration `json:"delay"`
	Bitrate int      `json:"bitrate"`
}

func (t tcConfig) apply() error {
	for _, role := range []string{"sender", "receiver"} {
		tcDelay := exec.Command(
			"docker",
			"exec",
			role,

			"tc",
			"qdisc",
			"add",
			"dev", "eth0",
			"root", "handle", "1:",
			"netem", "delay", fmt.Sprintf("%vms", t.Delay.Milliseconds()),
		)
		log.Printf("applying tcConfig: %v %v\n", tcDelay.Path, tcDelay.Args)
		tcDelay.Stdout = os.Stdout
		tcDelay.Stderr = os.Stderr

		err := tcDelay.Run()
		if err != nil {
			return err
		}

		tcBitrate := exec.Command(
			"docker",
			"exec",
			role,

			"tc",
			"qdisc", "add",
			"dev", "eth0",
			"parent", "1:", "handle", "2:",
			"tbf", "rate", fmt.Sprintf("%v", t.Bitrate), "latency", "400ms", "burst", "20kB",
		)
		log.Printf("applying tcConfig: %v %v\n", tcBitrate.Path, tcBitrate.Args)
		tcBitrate.Stdout = os.Stdout
		tcBitrate.Stderr = os.Stderr

		err = tcBitrate.Run()
		if err != nil {
			return err
		}

	}
	return nil
}

type tcPhase struct {
	Duration Duration `json:"duration"`
	Config   tcConfig `json:"config"`
}

type trafficController struct {
	phases []tcPhase
}

func (t *trafficController) run(ctx context.Context) error {
	if len(t.phases) <= 0 {
		return nil
	}

	for _, p := range t.phases {
		err := p.Config.apply()
		if err != nil {
			return err
		}
		if p.Duration.Duration == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(p.Duration.Duration):
		}
	}
	return nil
}
