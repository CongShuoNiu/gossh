package scp

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// the following code is a modified version of https://github.com/gnicod/goscplib
// which follows https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works
//Constants

const (
	SCP_PUSH_BEGIN_FILE       = "C"
	SCP_PUSH_BEGIN_FOLDER     = "D"
	SCP_PUSH_BEGIN_END_FOLDER = "0"
	SCP_PUSH_END_FOLDER       = "E"
	SCP_PUSH_END              = "\x00"
)

type Scp struct {
	client *ssh.Client
}

type TransferStats struct {
	Bytes    int64
	Duration time.Duration
}

// GetPerm 返回 SCP 协议需要的低 9 位文件权限。
func GetPerm(f *os.File) (perm string) {
	fileStat, err := f.Stat()
	if err != nil {
		return "0644"
	}
	mod := fileStat.Mode()
	// if it's a directory there's high bits we want to ditch
	// only keep the low bits
	if mod > (1 << 9) {
		mod = mod % (1 << 9)
	}
	return fmt.Sprintf("%#o", uint32(mod))
}

// NewScp 基于已有 SSH 连接创建 SCP 客户端。
func NewScp(clientConn *ssh.Client) *Scp {
	return &Scp{
		client: clientConn,
	}
}

// PullFile 将远端普通文件拉取到本地目录，并把传输指标和错误返回给调用方。
func (scp *Scp) PullFile(srcpath, targetFile string) (TransferStats, error) {
	session, err := scp.client.NewSession()
	if err != nil {
		return TransferStats{}, err
	}
	defer session.Close()

	iw, err := session.StdinPipe()
	if err != nil {
		return TransferStats{}, fmt.Errorf("create scp input pipe: %w", err)
	}
	or, err := session.StdoutPipe()
	if err != nil {
		return TransferStats{}, fmt.Errorf("create scp output pipe: %w", err)
	}

	type transferResult struct {
		stats TransferStats
		err   error
	}

	resultCh := make(chan transferResult, 1)
	start := time.Now()
	go func() {
		fmt.Fprint(iw, "\x00")

		sr := bufio.NewReader(or)
		localFile := path.Join(srcpath, path.Base(targetFile))
		src, err := os.Create(localFile)
		if err != nil {
			resultCh <- transferResult{err: fmt.Errorf("create local file %s: %w", localFile, err)}
			return
		}
		defer src.Close()

		controlString, err := sr.ReadString('\n')
		if err != nil {
			resultCh <- transferResult{err: fmt.Errorf("read scp control line: %w", err)}
			return
		}
		if strings.HasPrefix(controlString, "C") {
			fmt.Fprint(iw, "\x00")
			controlParts := strings.Split(controlString, " ")
			if len(controlParts) < 2 {
				resultCh <- transferResult{err: fmt.Errorf("invalid scp control line: %q", controlString)}
				return
			}
			size, err := strconv.ParseInt(controlParts[1], 10, 64)
			if err != nil {
				resultCh <- transferResult{err: fmt.Errorf("parse scp file size: %w", err)}
				return
			}
			/*
				bar := pb.New(int(size))
				bar.Units = pb.U_BYTES
				bar.ShowSpeed = true
				bar.Start()
				rp := io.MultiReader(sr, bar)
				if n, ok := io.CopyN(src, rp, size); ok != nil || n < size {
			*/
			if n, ok := io.CopyN(src, sr, size); ok != nil || n < size {
				fmt.Fprint(iw, "\x02")
				resultCh <- transferResult{
					stats: TransferStats{Bytes: n},
					err:   fmt.Errorf("copy remote file %s: copied %d of %d bytes: %w", targetFile, n, size, ok),
				}
				return
			}
			//			bar.Finish()
			if _, err := sr.Read(make([]byte, 1)); err != nil {
				resultCh <- transferResult{stats: TransferStats{Bytes: size}, err: fmt.Errorf("read scp end marker: %w", err)}
				return
			}
			fmt.Fprint(iw, "\x00")
			resultCh <- transferResult{stats: TransferStats{Bytes: size}}
			return
		}
		fmt.Fprint(iw, "\x00")
		resultCh <- transferResult{}
	}()

	if err := session.Run(fmt.Sprintf("scp -f %s", shellQuote(targetFile))); err != nil {
		return TransferStats{Duration: time.Since(start)}, err
	}
	result := <-resultCh
	result.stats.Duration = time.Since(start)
	return result.stats, result.err
}

// PushFile 将本地普通文件推送到远端目录，并返回传输指标和错误。
func (scp *Scp) PushFile(src string, dest string) (TransferStats, error) {
	session, err := scp.client.NewSession()
	if err != nil {
		return TransferStats{}, err
	}
	defer session.Close()

	fileSrc, err := os.Open(src)
	if err != nil {
		return TransferStats{}, fmt.Errorf("open source file %s: %w", src, err)
	}
	defer fileSrc.Close()

	srcStat, err := fileSrc.Stat()
	if err != nil {
		return TransferStats{}, fmt.Errorf("stat source file %s: %w", src, err)
	}
	w, err := session.StdinPipe()
	if err != nil {
		return TransferStats{}, fmt.Errorf("create scp input pipe: %w", err)
	}

	type transferResult struct {
		stats TransferStats
		err   error
	}

	resultCh := make(chan transferResult, 1)
	start := time.Now()
	go func() {
		defer w.Close()

		// Print the file content
		//fmt.Fprintln(w, SCP_PUSH_BEGIN_FILE+GetPerm(fileSrc), srcStat.Size(), filepath.Base(dest))
		if _, err := fmt.Fprintln(w, SCP_PUSH_BEGIN_FILE+GetPerm(fileSrc), srcStat.Size(), filepath.Base(src)); err != nil {
			resultCh <- transferResult{err: fmt.Errorf("write scp file header: %w", err)}
			return
		}
		n, err := io.Copy(w, fileSrc)
		if err != nil {
			resultCh <- transferResult{
				stats: TransferStats{Bytes: n},
				err:   fmt.Errorf("copy source file %s: %w", src, err),
			}
			return
		}
		if _, err := fmt.Fprint(w, SCP_PUSH_END); err != nil {
			resultCh <- transferResult{
				stats: TransferStats{Bytes: n},
				err:   fmt.Errorf("write scp end marker: %w", err),
			}
			return
		}
		resultCh <- transferResult{stats: TransferStats{Bytes: n}}
	}()
	//if err := session.Run("/usr/bin/scp -rt " + filepath.Dir(dest)); err != nil {
	if err := session.Run("/usr/bin/scp -rt " + shellQuote(dest)); err != nil {
		return TransferStats{Duration: time.Since(start)}, err
	}
	result := <-resultCh
	result.stats.Duration = time.Since(start)
	return result.stats, result.err
}

// PushDir 将本地目录递归推送到远端目录，并返回累计传输指标和首个错误。
func (scp *Scp) PushDir(src string, dest string) (TransferStats, error) {
	session, err := scp.client.NewSession()
	if err != nil {
		return TransferStats{}, err
	}
	defer session.Close()

	folderSrc, err := os.Open(src)
	if err != nil {
		return TransferStats{}, fmt.Errorf("open source directory %s: %w", src, err)
	}
	defer folderSrc.Close()
	w, err := session.StdinPipe()
	if err != nil {
		return TransferStats{}, fmt.Errorf("create scp input pipe: %w", err)
	}

	type transferResult struct {
		stats TransferStats
		err   error
	}

	resultCh := make(chan transferResult, 1)
	start := time.Now()
	go func() {
		//w = os.Stdout
		defer w.Close()

		if _, err := fmt.Fprintln(w, SCP_PUSH_BEGIN_FOLDER+GetPerm(folderSrc), SCP_PUSH_BEGIN_END_FOLDER, filepath.Base(src)); err != nil {
			resultCh <- transferResult{err: fmt.Errorf("write scp directory header: %w", err)}
			return
		}
		bytes, err := lsDir(w, src)
		if err != nil {
			resultCh <- transferResult{stats: TransferStats{Bytes: bytes}, err: err}
			return
		}
		if _, err := fmt.Fprintln(w, SCP_PUSH_END_FOLDER); err != nil {
			resultCh <- transferResult{
				stats: TransferStats{Bytes: bytes},
				err:   fmt.Errorf("write scp directory end marker: %w", err),
			}
			return
		}
		resultCh <- transferResult{stats: TransferStats{Bytes: bytes}}
	}()
	if err := session.Run("/usr/bin/scp -qrt " + shellQuote(dest)); err != nil {
		return TransferStats{Duration: time.Since(start)}, err
	}
	result := <-resultCh
	result.stats.Duration = time.Since(start)
	return result.stats, result.err
}

// prepareFile 按 SCP 协议向远端写入单个本地文件内容。
func prepareFile(w io.WriteCloser, src string) (int64, error) {
	fileSrc, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open source file %s: %w", src, err)
	}
	defer fileSrc.Close()

	//Get file size
	srcStat, err := fileSrc.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat source file %s: %w", src, err)
	}
	// Print the file content
	if _, err := fmt.Fprintln(w, SCP_PUSH_BEGIN_FILE+GetPerm(fileSrc), srcStat.Size(), filepath.Base(src)); err != nil {
		return 0, fmt.Errorf("write scp file header: %w", err)
	}
	n, err := io.Copy(w, fileSrc)
	if err != nil {
		return n, fmt.Errorf("copy source file %s: %w", src, err)
	}
	if _, err := fmt.Fprint(w, SCP_PUSH_END); err != nil {
		return n, fmt.Errorf("write scp end marker: %w", err)
	}
	return n, nil
}

// lsDir 递归遍历目录并把文件树按 SCP 协议写入远端。
func lsDir(w io.WriteCloser, dir string) (int64, error) {
	fi, err := ioutil.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read source directory %s: %w", dir, err)
	}
	var bytes int64
	//parcours des dossiers
	for _, f := range fi {
		if f.IsDir() {
			folderSrc, err := os.Open(path.Join(dir, f.Name()))
			if err != nil {
				return bytes, fmt.Errorf("open source directory %s: %w", path.Join(dir, f.Name()), err)
			}
			defer folderSrc.Close()
			if _, err := fmt.Fprintln(w, SCP_PUSH_BEGIN_FOLDER+GetPerm(folderSrc), SCP_PUSH_BEGIN_END_FOLDER, f.Name()); err != nil {
				return bytes, fmt.Errorf("write scp directory header: %w", err)
			}
			n, err := lsDir(w, path.Join(dir, f.Name()))
			bytes += n
			if err != nil {
				return bytes, err
			}
			if _, err := fmt.Fprintln(w, SCP_PUSH_END_FOLDER); err != nil {
				return bytes, fmt.Errorf("write scp directory end marker: %w", err)
			}
		} else {
			n, err := prepareFile(w, path.Join(dir, f.Name()))
			bytes += n
			if err != nil {
				return bytes, err
			}
		}
	}
	return bytes, nil
}

// shellQuote 对远端 shell 参数做单引号转义，避免路径中的空格或特殊字符被解释。
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
