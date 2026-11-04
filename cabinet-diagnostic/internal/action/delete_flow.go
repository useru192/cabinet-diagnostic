package action

import (
	"context"
	"fmt"
	"strings"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
)

// DeleteFlow 执行一次「手动输入路径删除」的完整流程：
//
//	输入路径 -> 字符串校验 -> ls 预览 -> 输入完整文件名确认 -> rm -> 回读验证 -> 审计落盘
//
// 任何一步不满足都中止且不产生写动作。确认要求输入完整文件名而非 y/n，
// 避免"随手回车"式的误删。
//
// 返回 false 表示用户未输入路径（直接回车或输入流结束），调用方据此退出程序；
// 其余情况（含被拒、取消、失败）返回 true，表示可以继续删除下一个文件。
func DeleteFlow(ctx context.Context, r *adb.Runner, audit *Auditor, readline func() (string, bool)) bool {
	fmt.Println()
	fmt.Println("========== Delete Device File ==========")
	fmt.Println("[WARNING] The file is removed from the device permanently. There is no recycle bin.")
	fmt.Printf("Only single files under %s can be deleted.\n", config.DeleteRoot)
	fmt.Println("Directories, symlinks, wildcards and protected business files are refused.")
	fmt.Println()
	fmt.Print("File path to delete (empty line to go back):\n  > ")

	line, ok := readline()
	if !ok || strings.TrimSpace(line) == "" {
		fmt.Println("Nothing was deleted.")
		return false
	}
	path := strings.TrimSpace(line)

	if err := ValidateDeletePath(path); err != nil {
		fmt.Printf("[REFUSED] %v\n", err)
		fmt.Println("Nothing was deleted.")
		return true
	}

	// 从这一步开始产生审计记录：即使后面取消/失败，删除意图也要留痕。
	audit.log("Requested path : %s", path)

	del := DeleteFile{Path: path}
	pv, err := del.Describe(ctx, r)
	if err != nil {
		fmt.Printf("[ERROR] %v\n", err)
		fmt.Println("Nothing was deleted.")
		audit.log("Result         : FAILED at preview (%v)", err)
		return true
	}
	if pv.IsDir || pv.IsSymlink {
		kind := "a directory"
		if pv.IsSymlink {
			kind = "a symlink"
		}
		fmt.Printf("[REFUSED] %s is %s; only single files can be deleted.\n", path, kind)
		fmt.Println("Nothing was deleted.")
		audit.log("Result         : REFUSED (%s)", kind)
		return true
	}

	fmt.Println()
	fmt.Println("Preview:")
	fmt.Printf("  Path     : %s\n", pv.Path)
	fmt.Printf("  Size     : %s\n", humanBytes(pv.Size))
	fmt.Printf("  Owner    : %s\n", pv.Owner)
	fmt.Printf("  Modified : %s\n", pv.Modified)
	fmt.Println()
	audit.log("Preview        : size=%d owner=%s modified=%s", pv.Size, pv.Owner, pv.Modified)

	base := pathBase(path)
	fmt.Printf("Type the FULL file name (%s) to confirm deletion, anything else cancels:\n  > ", base)

	confirm, ok := readline()
	confirmed := ok && strings.TrimSpace(confirm) == base
	audit.log("Confirmation   : %t", confirmed)
	if !confirmed {
		fmt.Println("Cancelled. Nothing was deleted.")
		audit.log("Result         : CANCELLED by user")
		return true
	}

	res, err := del.Apply(ctx, r, true)
	switch {
	case err != nil:
		fmt.Printf("[ERROR] %v\n", err)
		if res.Detail != "" {
			fmt.Printf("        %s\n", res.Detail)
		}
		audit.log("Result         : FAILED (%v) %s", err, res.Detail)
	case res.Done:
		fmt.Printf("[OK]     %s deleted and verified.\n", path)
		audit.log("Result         : DELETED (verified by re-listing the path)")
	default:
		// rm 执行了但没有验证通过：如实报告，不得粉饰为成功。
		fmt.Printf("[ERROR] Deletion could not be verified: %s\n", res.Detail)
		audit.log("Result         : UNVERIFIED (%s)", res.Detail)
	}
	return true
}

// humanBytes 与 check 包的 humanSize 同规则，本包独立实现避免跨包依赖。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	val := float64(n)
	units := []string{"K", "M", "G", "T"}
	for _, u := range units {
		val /= unit
		if val < unit {
			return fmt.Sprintf("%.1f %s", val, u)
		}
	}
	return fmt.Sprintf("%.1f P", val/unit)
}
