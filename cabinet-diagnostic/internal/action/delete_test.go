package action

import "testing"

func TestValidateDeletePath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// 合法：根目录下的普通文件
		{"plain log file", "/data/cabinet/app.log", false},
		// 合法：子目录内的文件（含大写目录名）
		{"nested file", "/data/cabinet/CabinetControlgo/cabinet.log.20261027", false},
		// 合法：upgrade 目录下未受保护的文件
		{"upgrade dir file", "/data/cabinet/upgrade/ota.bin", false},
		// 合法：点号、下划线、连字符
		{"dotted name", "/data/cabinet/backup.old.tar.gz", false},
		{"underscore", "/data/cabinet/cabinet_control-go_v1.2", false},

		// 拒绝：根目录本身
		{"root itself", "/data/cabinet", true},
		{"root with slash", "/data/cabinet/", true},
		// 拒绝：白名单之外
		{"outside root", "/etc/passwd", true},
		{"sibling of root", "/data/other/file.log", true},
		{"relative path", "data/cabinet/app.log", true},
		{"empty", "", true},
		{"spaces only", "   ", true},
		// 拒绝：目录穿越与空段
		{"traversal", "/data/cabinet/../cabinet/app.log", true},
		{"trailing parent", "/data/cabinet/..", true},
		{"double slash", "/data/cabinet//app.log", true},
		// 拒绝：特殊字符（空格、通配符、引号、命令分隔符）
		{"space in name", "/data/cabinet/my file.log", true},
		{"wildcard star", "/data/cabinet/*.log", true},
		{"wildcard question", "/data/cabinet/?.log", true},
		{"semicolon injection", "/data/cabinet/a;reboot", true},
		{"dollar injection", "/data/cabinet/$PATH", true},
		{"backslash", "/data/cabinet\\app.log", true},
		{"chinese char", "/data/cabinet/日志.txt", true},
		// 拒绝：保护清单命中
		{"protected sqlite", "/data/cabinet/sqlite.db", true},
		{"protected deviceid", "/data/cabinet/deviceid", true},
		{"protected binary", "/data/cabinet/cabinetcontrolgo", true},
		{"protected script", "/data/cabinet/restart.sh", true},
		{"protected apk", "/data/cabinet/cabinet.apk", true},
		// 保护清单按基名匹配、全路径生效：子目录里同名文件同样被拒（宁可保守）
		{"same name in subdir", "/data/cabinet/CabinetControlgo/restart.sh", true},
		// 基名不在清单内的备份文件不误伤
		{"backup suffix", "/data/cabinet/restart.sh.bak", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateDeletePath(c.path)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateDeletePath(%q) error = %v, wantErr %v", c.path, err, c.wantErr)
			}
		})
	}
}

func TestPathBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/data/cabinet/app.log", "app.log"},
		{"/data/cabinet/CabinetControlgo/x.log", "x.log"},
		{"/data/cabinet/", ""},
		{"app.log", "app.log"},
	}
	for _, c := range cases {
		if got := pathBase(c.in); got != c.want {
			t.Errorf("pathBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 K"},
		{20110464, "19.2 M"},
		{10 << 30, "10.0 G"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
