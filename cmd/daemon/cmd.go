package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"osdman/pkg/config"
	"osdman/pkg/consts"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var Command = &cobra.Command{
	Use:   "daemon",
	Short: "Run a osdman daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ok := cmd.Context().Value(consts.CtxVarConfig).(*config.Config)
		if !ok {
			return errors.New("configuration not passed in context")
		}

		ah, err := newAppHandle(cfg)
		if err != nil {
			return err
		}

		ah.Run(cmd.Context())
		return nil
	},
}

type appHandle struct {
	cfg           *config.Config
	listener      *net.UnixConn
	runtimeSocket string
	pipeReader    *io.PipeReader
	pipeWriter    *io.PipeWriter

	cancelSignalCtx context.Context
	cancelFuncs     []func()
}

func newAppHandle(cfg *config.Config) (*appHandle, error) {
	runtimeSocket := fmt.Sprintf("/run/user/%d/osdcmd.sock", os.Getuid())
	ln, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: runtimeSocket, Net: "unixgram"})
	if err != nil {
		return nil, err
	}

	ah := &appHandle{
		cfg:           cfg,
		listener:      ln,
		runtimeSocket: runtimeSocket,
	}

	var cancelFunc func()
	ah.pipeReader, ah.pipeWriter = io.Pipe()
	ah.cancelSignalCtx, cancelFunc = signal.NotifyContext(
		context.Background(),
		os.Interrupt, os.Kill,
	)
	ah.cancelFuncs = append(
		ah.cancelFuncs,
		func() {
			log.Println("info: closing listener")
			ah.listener.Close()
			os.Remove(ah.runtimeSocket)
			log.Println("info: closing pipes")
			ah.pipeReader.Close()
			ah.pipeWriter.Close()
		},
		cancelFunc,
	)

	return ah, nil
}

func (a *appHandle) Run(ctx context.Context) {
	go func() {
		<-a.cancelSignalCtx.Done()
		a.Close()
	}()

	wg := &sync.WaitGroup{}

	wg.Add(2)
	go a.runDomainSocket(wg)
	go a.runWobPipe(wg)
	wg.Wait()
}

func (a *appHandle) runDomainSocket(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		var buf [1024]byte
		n, _, err := a.listener.ReadFromUnix(buf[:])
		if err != nil {
			break
		}

		a.handleConn(buf[:n])
	}
}

func (a *appHandle) runWobPipe(wg *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())

	wobCmd := exec.CommandContext(ctx, "wob")
	wobOut := os.Stdout

	wobCmd.Stdin = a.pipeReader
	wobCmd.Stdout = wobOut
	wobCmd.Stderr = wobOut

	a.cancelFuncs = append(a.cancelFuncs, func() {
		log.Println("info: stopping wob process")
		cancel()
		log.Println("info: stopped wob process")
	})

	err := wobCmd.Run()
	if err != nil {
		log.Printf("error: wob not running: %s", err)
	}
	wg.Done()
}

func (a *appHandle) Close() {
	for _, fn := range a.cancelFuncs {
		fn()
	}
}

func (a *appHandle) handleConn(data []byte) {
	splits := strings.SplitN(strings.TrimSpace(string(data)), "/", 2)
	if len(splits) != 2 {
		log.Printf("warning: invalid command pattern sent: %s", data)
		return
	}

	domain, verb := splits[0], splits[1]
	domainCfg, ok := a.cfg.Domains[domain]
	if !ok {
		log.Printf("error: unknown domain sent: %s", domain)
		return
	}

	verbCfg, ok := domainCfg.Verbs[verb]
	if !ok {
		log.Printf("error: verb not found: %s", verb)
		return
	}

	_, err := runShellCommand(verbCfg.Command)
	if err != nil {
		log.Printf("error: error while running %s/%s command: %s", domain, verb, err)
		return
	}

	output, err := runShellCommand(domainCfg.StatCmd)
	if err != nil {
		log.Printf("error: error while running %s/stat command: %s", domain, err)
		return
	}

	d, err := strconv.ParseFloat(output, 64)
	if err != nil {
		log.Printf("error: invalid numeric output from stat_cmd: %s/%s : %s", domain, verb, output)
	}

	a.pipeWriter.Write([]byte(fmt.Sprintf("%d\n", int(d))))
}

func runShellCommand(shellCommand string) (string, error) {
	output := bytes.NewBuffer([]byte{})

	cmd := exec.Command("bash", "-c", shellCommand)
	cmd.Stdout = output

	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}
