// Copyright 2018 gossh Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Author: andes
// Email: email.tata@qq.com

package tools

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const colorListShell = `gossh_print_entry() {
  item="$1"
  name="${item##*/}"
  if [ -L "$item" ]; then
    printf '\033[36m%s@\033[0m\n' "$name"
  elif [ -d "$item" ]; then
    printf '\033[34m%s/\033[0m\n' "$name"
  elif [ -x "$item" ]; then
    printf '\033[32m%s*\033[0m\n' "$name"
  elif gossh_is_script "$item" "$name"; then
    printf '\033[31m%s\033[0m\n' "$name"
  else
    printf '\033[37m%s\033[0m\n' "$name"
  fi
}
gossh_is_script() {
  case "$2" in
    *.sh|*.bash|*.py|*.pl|*.rb|*.awk|*.sed) return 0 ;;
  esac
  head -c 2 "$1" 2>/dev/null | grep -q '^#!'
}
gossh_color_ls() {
  for target in "$@"; do
    if [ -d "$target" ]; then
      for item in "$target"/*; do
        [ -e "$item" ] || continue
        gossh_print_entry "$item"
      done
    else
      [ -e "$target" ] && gossh_print_entry "$target"
    fi
  done
}
gossh_color_ls`

// CriticalPaths 定义了受保护的系统关键路径，
// 删除这些路径可能导致系统不可用。
var CriticalPaths = []string{
	"/",
	"/bin",
	"/boot",
	"/dev",
	"/etc",
	"/home",
	"/lib",
	"/lib64",
	"/media",
	"/mnt",
	"/opt",
	"/proc",
	"/root",
	"/run",
	"/sbin",
	"/srv",
	"/sys",
	"/usr",
	"/var",
}

// check the comand safe
// true:safe false:refused
func CheckSafe(cmd string, blacks []string) bool {
	lcmd := strings.ToLower(cmd)
	cmds := strings.Split(lcmd, " ")
	for _, ds := range cmds {
		for _, bk := range blacks {
			if ds == bk {
				return false
			}
		}
	}
	return true
}

// EnhanceListCommand 将简单 ls 命令增强为按文件类型着色输出。
// 目录显示蓝色，普通文件显示白色，可执行文件显示绿色，未授权脚本显示红色。
func EnhanceListCommand(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	parts := strings.Fields(trimmed)
	if len(parts) == 0 || parts[0] != "ls" || hasShellMeta(trimmed) {
		return cmd
	}

	args := parts[1:]
	if hasListOptions(args) {
		return enhanceGNUListCommand(args)
	}
	if len(args) == 0 {
		args = []string{"."}
	}

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}
	return colorListShell + " " + strings.Join(quotedArgs, " ")
}

// hasListOptions 判断 ls 参数中是否包含选项；包含选项时优先保留原 ls 语义。
func hasListOptions(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

// enhanceGNUListCommand 使用 GNU ls 原生能力为带选项的 ls 命令开启颜色和类型后缀。
func enhanceGNUListCommand(args []string) string {
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}
	return "LS_COLORS='di=01;34:ln=01;36:ex=01;32:*.sh=01;31:*.bash=01;31:*.py=01;31:*.pl=01;31:*.rb=01;31' ls --color=always -F " + strings.Join(quotedArgs, " ")
}

// hasShellMeta 判断命令中是否包含 shell 控制符，避免改变管道、重定向等复杂命令语义。
func hasShellMeta(cmd string) bool {
	return strings.ContainsAny(cmd, "|;&<>`$(){}[]*?")
}

// shellQuote 对远端 shell 参数做单引号转义，避免路径中的空格或特殊字符被解释。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// IsRmCommand 判断命令是否为删除操作（rm 或 unlink）。
func IsRmCommand(cmd string) bool {
	lcmd := strings.ToLower(strings.TrimSpace(cmd))
	parts := strings.Fields(lcmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == "rm" || parts[0] == "unlink"
}

// ExtractRmPaths 从 rm 命令中提取目标路径参数，
// 跳过命令名和以 "-" 开头的选项。
func ExtractRmPaths(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return nil
	}
	var paths []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "-") {
			continue
		}
		paths = append(paths, p)
	}
	return paths
}

// IsCriticalPath 判断路径是否为系统关键路径或其直接子路径。
// 例如 "/"、"/etc" 为关键路径，"/etc/ssh" 也视为关键。
func IsCriticalPath(path string) bool {
	clean := filepath.Clean(path)
	for _, cp := range CriticalPaths {
		if clean == cp {
			return true
		}
		// 检测是否为关键路径的子路径（如 /etc/ssh）
		if strings.HasPrefix(clean, cp+"/") {
			return true
		}
	}
	return false
}

// FindCriticalPaths 从路径列表中筛选出关键路径。
func FindCriticalPaths(paths []string) []string {
	var critical []string
	for _, p := range paths {
		if IsCriticalPath(p) {
			critical = append(critical, p)
		}
	}
	return critical
}

// ConfirmDelete 针对删除操作进行预览和二次确认。
// 返回 true 表示用户确认执行，false 表示取消。
func ConfirmDelete(cmd string) bool {
	paths := ExtractRmPaths(cmd)
	critical := FindCriticalPaths(paths)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║               DANGEROUS DELETE OPERATION                ║")
	fmt.Println("╠══════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Command: %s\n", padRight(cmd, 52))
	fmt.Println("╠══════════════════════════════════════════════════════════╣")

	if len(critical) > 0 {
		fmt.Println("║ ⚠  CRITICAL SYSTEM PATH(S) DETECTED:                   ║")
		for _, p := range critical {
			fmt.Printf("║   • %s\n", padRight(p, 50))
		}
		fmt.Println("║                                                        ║")
		fmt.Println("║   Deleting these paths may RENDER THE SYSTEM UNUSABLE!  ║")
		fmt.Println("╠══════════════════════════════════════════════════════════╣")
	}

	if len(paths) > 0 {
		fmt.Println("║ Target path(s):                                        ║")
		for _, p := range paths {
			fmt.Printf("║   • %s\n", padRight(p, 50))
		}
	} else {
		fmt.Println("║ (No explicit paths detected — command may use globs)    ║")
	}
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Print("Type 'yes' to confirm deletion, anything else to cancel: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input, operation cancelled.")
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "yes"
}

// padRight 右填充字符串到指定长度（用于对齐显示）。
func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}

//check path is exit

func FileExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	} else {
		return !fi.IsDir()
	}
}

func PathExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	} else {
		return fi.IsDir()
	}
}

func MakePath(path string) error {
	if FileExists(path) {
		return errors.New(path + " is a normal file ,not a dir")
	}

	if !PathExists(path) {
		return os.MkdirAll(path, os.ModePerm)
	} else {
		return nil
	}
}
