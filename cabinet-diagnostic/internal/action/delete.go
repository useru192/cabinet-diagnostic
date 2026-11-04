package action

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
	"cabinet-diagnostic/parsers"
)

// ---------------------------------------------------------------
// 路径校验
// ---------------------------------------------------------------

// ValidateDeletePath 校验删除路径，拒绝一切白名单之外或危险的输入。
//
// 规则：
//  1. 必须位于 config.DeleteRoot 之下，且不能是根目录本身；
//  2. 只允许字母数字与 . _ - /（因此拼进 shell 命令不可能被注入）；
//  3. 拒绝目录穿越（..）与空路径段（//）；
//  4. 基名不得命中保护清单（可执行程序、配置、业务数据、脚本全部禁删）。
//
// 目录与符号链接的拒绝在 Describe/Apply 中基于 ls 结果判断，这里只管字符串。
func ValidateDeletePath(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return errors.New("empty path")
	}
	if p == config.DeleteRoot || p == config.DeleteRoot+"/" {
		return fmt.Errorf("refusing to delete the root directory %s itself", config.DeleteRoot)
	}

	prefix := config.DeleteRoot + "/"
	if !strings.HasPrefix(p, prefix) {
		return fmt.Errorf("path must start with %s (only files under this directory can be deleted)", prefix)
	}

	for _, c := range p {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == '/':
		default:
			return fmt.Errorf("character %q is not allowed in the path (no spaces or wildcards)", c)
		}
	}

	if strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") || strings.Contains(p, "//") {
		return errors.New("path traversal or empty segment is not allowed")
	}

	base := pathBase(p)
	for _, f := range config.DeleteProtectedFiles {
		if base == f {
			return fmt.Errorf("%s is a protected business file, deletion refused", base)
		}
	}
	return nil
}

// pathBase 取最后一个 "/" 之后的基名。
// 不用 filepath.Base：宿主机是 Windows，对 "/" 的处理不可依赖。
func pathBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ---------------------------------------------------------------
// 删除操作
// ---------------------------------------------------------------

// DeleteFile 删除设备上的一个普通文件。
type DeleteFile struct {
	Path string
}

func (d DeleteFile) Name() string { return "Delete File" }

// Describe 用 ls -ld 预览目标。不产生任何写动作。
func (d DeleteFile) Describe(ctx context.Context, r *adb.Runner) (Preview, error) {
	if err := ValidateDeletePath(d.Path); err != nil {
		return Preview{}, err
	}

	out, err := r.Shell(ctx, config.TimeoutCommon, "ls -ld "+d.Path)
	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			return Preview{}, fmt.Errorf("ls timed out after %s", config.TimeoutCommon)
		}
		return Preview{}, err
	}

	combined := out.Combined()
	if strings.Contains(strings.ToLower(combined), "no such file") {
		return Preview{}, fmt.Errorf("file not found: %s", d.Path)
	}

	entries, _ := parsers.ParseLsLong(combined)
	if len(entries) == 0 {
		return Preview{}, fmt.Errorf("cannot parse ls output for %s", d.Path)
	}

	e := entries[0]
	return Preview{
		Path:      d.Path,
		IsDir:     e.Type == parsers.EntryDir,
		IsSymlink: e.Type == parsers.EntrySymlink,
		Size:      e.Size,
		Owner:     e.Owner,
		Modified:  e.Date + " " + e.Time,
	}, nil
}

// Apply 执行删除。rm 成功且回读确认文件已消失才返回 Done=true。
//
// 不使用 rm -f / rm -r：不递归、不强删，目录与符号链接一律拒绝。
func (d DeleteFile) Apply(ctx context.Context, r *adb.Runner, confirm bool) (Result, error) {
	if !confirm {
		return Result{Done: false, Detail: "cancelled"}, nil
	}
	if err := ValidateDeletePath(d.Path); err != nil {
		return Result{}, err
	}

	// 动手前再核对一次目标：仍然是普通文件才删（防止预览后被替换成目录/链接）。
	pv, err := d.Describe(ctx, r)
	if err != nil {
		return Result{}, err
	}
	if pv.IsDir || pv.IsSymlink {
		return Result{}, errors.New("target is a directory or symlink, refusing to delete")
	}

	out, err := r.Shell(ctx, config.TimeoutDelete, "rm "+d.Path)
	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			return Result{}, fmt.Errorf(
				"rm timed out after %s; the file may or may not be deleted, please re-check", config.TimeoutDelete)
		}
		return Result{}, err
	}
	if out.ExitCode != 0 {
		return Result{Done: false, Detail: strings.TrimSpace(out.Combined())},
			fmt.Errorf("rm exited with code %d", out.ExitCode)
	}

	// 回读验证：rm 报成功但文件还在属于异常，必须如实上报，不得默认成功。
	verify, verr := r.Shell(ctx, config.TimeoutCommon, "ls -ld "+d.Path)
	if verr != nil {
		return Result{Done: false, Detail: "deleted, but verification could not run: " + verr.Error()}, nil
	}
	if !strings.Contains(strings.ToLower(verify.Combined()), "no such file") {
		return Result{Done: false, Detail: "rm reported success but the file still exists"}, nil
	}

	return Result{Done: true, Detail: "deleted and verified"}, nil
}
