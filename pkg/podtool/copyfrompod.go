package podtool

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/jumpserver/koko/pkg/logger"
)

func (p *PodTool) CopyFromPod(filePath string, destPath string) error {
	reader, outStream := io.Pipe()

	cmdPath := "/tmp" + filePath

	p.ExecConfig = ExecConfig{
		// Command: []string{"tar", "cf", "-", cmdPath},
		Command: []string{"sh", "-c", "cat " + cmdPath},
		// Command:     []string{"sh", "-c", "dd if=" + cmdPath + " of=/dev/stdout"},
		Stdin:  os.Stdin,
		Stdout: outStream,
		// Stderr:      os.Stderr,
		StdoutClose: outStream,
		// NoPreserve:  true,
	}
	var stderr bytes.Buffer
	p.ExecConfig.Stderr = &stderr

	// eof := errors.New("io.EOF")
	err := p.Exec(Download)
	if err != nil {
		if len(stderr.Bytes()) != 0 {
			logger.Debug("STDERR: %s", stderr.String())
			return fmt.Errorf("STDERR: %s", stderr.String())
		}
		return err
	}
	// logger.Debug("print download err:", err)
	// if err == eof {
	// 	outStream.Close()
	// 	err = nil
	// }
	// if err != nil && err != eof {
	// 	return err
	// 	// return nil, err
	// }

	// defer outStream.Close()

	// file, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	file, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	// defer file.Close()
	if err != nil {
		return err
		// return nil, err
	}

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
		// return nil, err
	}

	// r := bufio.NewReader(reader)
	// w := bufio.NewWriter(file)
	// size := 4 * 1024
	// buf := make([]byte, 4*1024)
	// logger.Debug("错误出在哪里:", "11111-------", err)
	// for {
	// 	n, err := r.Read(buf)
	// 	// logger.Debug("错误出在哪里:", "22222", n)
	// 	if err != nil && err != io.EOF {
	// 		return err
	// 		// return nil, err
	// 	}
	// 	if n == 0 {
	// 		break
	// 	}
	// 	_, err = w.Write(buf[:n])
	// 	if err != nil {
	// 		// return nil, err
	// 		return err
	// 	}
	// 	if n < size {
	// 		break
	// 	}
	// }
	// // // logger.Debug("错误出在哪里:", "3333", err)
	// err = w.Flush()
	// if err != nil {
	// 	return err
	// 	// return nil, err
	// }

	// return reader, err
	return err
}
