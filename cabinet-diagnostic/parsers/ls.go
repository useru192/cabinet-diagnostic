package parsers

import (
	"strconv"
	"strings"
)

// EntryType 是目录项的类型。
type EntryType int

const (
	EntryFile EntryType = iota
	EntryDir
	EntrySymlink
	EntryOther
)

func (t EntryType) String() string {
	switch t {
	case EntryFile:
		return "file"
	case EntryDir:
		return "directory"
	case EntrySymlink:
		return "symlink"
	default:
		return "other"
	}
}

// FileEntry 是 ls -la 输出的一行。
//
// 表头示例：
//
//	-rwxrwxrwx  1 root   root   20110464 2024-05-17 11:37 cabinet.apk
//	drwxrwxrwx 17 shell  shell      3452 2024-04-19 12:35 CabinetControlgo
//
// 字段：mode links owner group size date time name
// 注意 owner/group 宽度可变且字段间可能有多于一个空格，必须用 Fields 归一化。
type FileEntry struct {
	Mode  string
	Links int
	Owner string
	Group string
	Size  int64
	Date  string
	Time  string
	Name  string
	Type  EntryType
}

// ParseLsLong 解析 ls -la 的输出（含 total 行）。
// 返回无法解析的行（不含 total 行与空行）。
func ParseLsLong(stdout string) (entries []FileEntry, unparsed []string) {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "total ") {
			continue
		}

		f := strings.Fields(line)
		if len(f) < 8 {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		size, err := strconv.ParseInt(f[4], 10, 64)
		if err != nil {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		links, _ := strconv.Atoi(f[1])

		name := strings.Join(f[7:], " ")
		typ := entryTypeOf(f[0])
		if typ == EntrySymlink {
			// "name -> target"，只保留 name
			if i := strings.Index(name, " -> "); i >= 0 {
				name = name[:i]
			}
		}

		entries = append(entries, FileEntry{
			Mode:  f[0],
			Links: links,
			Owner: f[2],
			Group: f[3],
			Size:  size,
			Date:  f[5],
			Time:  f[6],
			Name:  name,
			Type:  typ,
		})
	}
	return entries, unparsed
}

func entryTypeOf(mode string) EntryType {
	if mode == "" {
		return EntryOther
	}
	switch mode[0] {
	case '-':
		return EntryFile
	case 'd':
		return EntryDir
	case 'l':
		return EntrySymlink
	default:
		return EntryOther
	}
}
