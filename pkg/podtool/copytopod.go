package podtool

import (
	"bytes"
	"fmt"

	"github.com/jumpserver/koko/pkg/logger"
)

func (p *PodTool) CopyToContainer(destPath string) error {
	// if p.ExecConfig.NoPreserve {
	// 	p.ExecConfig.Command = []string{"tar", "--no-same-permissions", "--no-same-owner", "-xmf", "-"}
	// } else {
	// 	p.ExecConfig.Command = []string{"tar", "-xmf", "-"}
	// }
	// if len(destPath) > 0 {
	// 	p.ExecConfig.Command = append(p.ExecConfig.Command, "-C", destPath)
	// }

	// p.ExecConfig.Command = []string{"dd", "if=/dev/stdin", "of=/tmp/" + destPath}
	p.ExecConfig.Command = []string{"sh", "-c", "cat /dev/stdin >> /tmp/" + destPath}
	logger.Debug("print dd:", p.ExecConfig.Command)
	p.ExecConfig.Tty = false
	var stderr bytes.Buffer
	p.ExecConfig.Stderr = &stderr
	err := p.Exec(Exec)
	var stdout bytes.Buffer
	// p.ExecConfig.Stdout = &stdout
	if err != nil {
		result := ""
		if len(stdout.Bytes()) != 0 {
			result = stdout.String()
		}
		if len(stderr.Bytes()) != 0 {
			result = stderr.String()
		}
		return fmt.Errorf(err.Error(), result)
	}
	return nil
}
