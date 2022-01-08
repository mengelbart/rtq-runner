package dockertest

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const setupTimeout = 10 * time.Second

var implementations = map[string]struct {
	sender       string
	senderArgs   string
	receiver     string
	receiverArgs string
}{
	"pion-gcc": {
		sender:       "engelbart/bwe-test-pion",
		senderArgs:   "",
		receiver:     "engelbart/bwe-test-pion",
		receiverArgs: "",
	},
	"rtp-over-quic-udp-gcc": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--transport udp --cc-dump /log/gcc.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --gcc --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--transport udp --rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --twcc --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-udp-scream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--transport udp --cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--transport udp --rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --rfc8888 --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-gcc": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/gcc.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --gcc --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --twcc --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --rfc8888 --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-gcc-newreno": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/gcc.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --gcc --newreno --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --twcc --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream-newreno": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --newreno --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --rfc8888 --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream-local-feedback": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --local-rfc8888 --codec h264 --source /input/input.y4m",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-gcc-stream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/gcc.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --gcc --codec h264 --source /input/input.y4m --stream",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --twcc --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream-stream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --codec h264 --source /input/input.y4m --stream",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --rfc8888 --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-gcc-newreno-stream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/gcc.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --gcc --newreno --codec h264 --source /input/input.y4m --stream",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --twcc --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream-newreno-stream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --newreno --codec h264 --source /input/input.y4m --stream",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --rfc8888 --codec h264 --sink /output/output.mkv",
	},
	"rtp-over-quic-scream-local-feedback-stream": {
		sender:       "engelbart/rtp-over-quic",
		senderArgs:   "--cc-dump /log/scream.log --rtcp-dump /log/rtcp_in.log --rtp-dump /log/rtp_out.log --qlog /log --scream --local-rfc8888 --codec h264 --source /input/input.y4m --stream",
		receiver:     "engelbart/rtp-over-quic",
		receiverArgs: "--rtcp-dump /log/rtcp_out.log --rtp-dump /log/rtp_in.log --qlog /log --codec h264 --sink /output/output.mkv",
	},
}

func TestOne(t *testing.T) {
	composeFile := "1-docker-compose.yml"

	leftPhases := []tcPhase{
		{
			Duration: 40 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "2500000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "600000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 0,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
	}
	rightPhases := []tcPhase{
		{
			Duration: 40 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "2500000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "600000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 20 * time.Second,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
		{
			Duration: 0,
			Config: tcConfig{
				Delay:   50 * time.Millisecond,
				Jitter:  30 * time.Millisecond,
				Rate:    "1000000",
				Burst:   "10kb",
				Latency: 300 * time.Millisecond,
			},
		},
	}

	date := time.Now().Unix()
	outputBase := path.Join("output/", fmt.Sprintf("%v", date))
	plotBase := path.Join("html/", fmt.Sprintf("%v", date))

	for name, implementation := range implementations {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			outputDir := path.Join(outputBase, name, "1")
			assert.NoError(t, os.MkdirAll(outputDir, 0755))

			leftRouterLog, err := os.Create(path.Join(outputDir, "leftrouter.log"))
			assert.NoError(t, err)
			rightRouterLog, err := os.Create(path.Join(outputDir, "rightrouter.log"))
			assert.NoError(t, err)
			err = createNetwork(ctx, composeFile, leftPhases, rightPhases, leftRouterLog, rightRouterLog)
			assert.NoError(t, err)

			defer func() {
				if err := teardown(ctx, composeFile); err != nil {
					log.Fatal(err)
				}
			}()

			cmd := exec.Command(
				"docker-compose", "-f", composeFile, "up", "--abort-on-container-exit",
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			cmd.Env = os.Environ()

			for k, v := range map[string]string{
				"SENDER":        implementation.sender,
				"SENDER_ARGS":   implementation.senderArgs,
				"RECEIVER":      implementation.receiver,
				"RECEIVER_ARGS": implementation.receiverArgs,
				"OUTPUT":        outputDir,
			} {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
			}

			if err = cmd.Start(); err != nil {
				assert.NoError(t, err)
			}

			errCh := make(chan error)
			go func() {
				errCh <- cmd.Wait()
			}()
			select {
			case <-time.After(120 * time.Second):
			case err = <-errCh:
				assert.NoError(t, err)
			}

			plotDir := path.Join(plotBase, name, "1")
			assert.NoError(t, os.MkdirAll(plotDir, 0755))

			plotCMD := exec.Command(
				"./plot.py",
				"rates",
				"--input_dir", outputDir,
				"--output_dir", plotDir,
				"--basetime", fmt.Sprintf("%v", date),
				"--router", "leftrouter.log",
			)
			fmt.Println(plotCMD.Args)
			plotCMD.Stderr = os.Stderr
			plotCMD.Stdout = os.Stdout
			assert.NoError(t, plotCMD.Run())

			plotCMD = exec.Command(
				"./plot.py",
				"qlog-cwnd",
				"--input_dir", outputDir,
				"--output_dir", plotDir,
				"--basetime", fmt.Sprintf("%v", date),
			)
			fmt.Println(plotCMD.Args)
			plotCMD.Stderr = os.Stderr
			plotCMD.Stdout = os.Stdout
			assert.NoError(t, plotCMD.Run())

			plotCMD = exec.Command(
				"./plot.py",
				"qlog-in-flight",
				"--input_dir", outputDir,
				"--output_dir", plotDir,
				"--basetime", fmt.Sprintf("%v", date),
			)
			fmt.Println(plotCMD.Args)
			plotCMD.Stderr = os.Stderr
			plotCMD.Stdout = os.Stdout
			assert.NoError(t, plotCMD.Run())

			plotCMD = exec.Command(
				"./plot.py",
				"qlog-rtt",
				"--input_dir", outputDir,
				"--output_dir", plotDir,
				"--basetime", fmt.Sprintf("%v", date),
			)
			fmt.Println(plotCMD.Args)
			plotCMD.Stderr = os.Stderr
			plotCMD.Stdout = os.Stdout
			assert.NoError(t, plotCMD.Run())

			htmlCMD := exec.Command(
				"./plot.py",
				"html",
				"--output_dir", plotDir,
			)
			fmt.Println(htmlCMD.Args)
			htmlCMD.Stderr = os.Stderr
			htmlCMD.Stdout = os.Stdout
			assert.NoError(t, htmlCMD.Run())
		})
	}
}

func createNetwork(
	ctx context.Context,
	composeFile string,
	leftPhases []tcPhase,
	rightPhases []tcPhase,
	leftRouterLog io.Writer,
	rightRouterLog io.Writer,
) error {
	cmd := exec.Command(
		"docker-compose", "-f", composeFile, "up", "--force-recreate", "leftrouter", "rightrouter",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	lrShaper, err := newTrafficShaper(ctx, "/leftrouter", leftPhases, leftRouterLog)
	if err != nil {
		return err
	}

	rrShaper, err := newTrafficShaper(ctx, "/rightrouter", rightPhases, rightRouterLog)
	if err != nil {
		return err
	}

	go lrShaper.run(ctx)
	go rrShaper.run(ctx)

	return nil
}

func teardown(ctx context.Context, composeFile string) error {
	downCMD := exec.Command("docker-compose", "-f", composeFile, "down")
	downCMD.Stdout = os.Stdout
	downCMD.Stderr = os.Stderr

	// Use host env
	downCMD.Env = os.Environ()
	if err1 := downCMD.Run(); err1 != nil {
		log.Printf("failed to shutdown docker compose setup: %v\n", err1)
	}
	return nil
}
