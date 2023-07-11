package podtool

import (
	"bytes"
	"io"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type PodTool struct {
	Namespace     string
	PodName       string
	ContainerName string
	K8sClient     *kubernetes.Clientset
	RestClient    *rest.Config
	ExecConfig    ExecConfig
}

type CopyOptions struct {
	PodName    string
	Container  string
	Namespace  string
	NoPreserve bool
	MaxTries   int

	ClientConfig *rest.Config
	Clientset    *kubernetes.Clientset
	// Clientset         kubernetes.Interface
	ExecParentCmdName string
	ExecConfig        ExecConfig

	args []string

	genericclioptions.IOStreams
}

type ExecConfig struct {
	Command     []string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	StdoutClose io.WriteCloser
	Tty         bool
	NoPreserve  bool
}

func (p *PodTool) ExecCommand(commands []string) ([]byte, error) {
	var stdout bytes.Buffer
	p.ExecConfig.Stdout = &stdout
	p.ExecConfig.Command = commands
	err := p.Exec(Exec)
	if err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

type ActionType string

const Exec ActionType = "Exec"
const Download ActionType = "Download"

// Exec 在给定容器中执行命令
func (p *PodTool) Exec(actionType ActionType) error {

	config := p.ExecConfig
	req := p.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(p.PodName).
		Namespace(p.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   config.Command,
			Container: p.ContainerName,
			Stdin:     config.Stdin != nil,
			Stdout:    config.Stdout != nil,
			Stderr:    config.Stderr != nil,
			TTY:       config.Tty,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(p.RestClient, "POST", req.URL())
	if err != nil {
		return err
	}
	if actionType == Download {
		go func() {
			defer p.ExecConfig.StdoutClose.Close()
			_ = p.stream(exec)
		}()
	} else {
		err = p.stream(exec)
	}
	return err
	// var errStream error
	// if actionType == Download {
	// 	go func() {
	// 		errStream = p.stream(exec)
	// 		// if errStream == nil {
	// 		// 	errStream = errors.New("io.EOF")
	// 		// }
	// 		logger.Debug("执行了几次:", errStream)
	// 		// defer p.ExecConfig.StdoutClose.Close()
	// 	}()
	// } else {
	// 	errStream = p.stream(exec)
	// }
	// return errStream
}

func (p *PodTool) stream(exec remotecommand.Executor) error {
	config := p.ExecConfig
	var sizeQueue remotecommand.TerminalSizeQueue
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:             config.Stdin,
		Stdout:            config.Stdout,
		Stderr:            config.Stderr,
		Tty:               config.Tty,
		TerminalSizeQueue: sizeQueue,
	})
}
