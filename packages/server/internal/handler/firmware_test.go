package handler

import "testing"

// TestParseVersion 校验固件版本号解析
func TestParseVersion(t *testing.T) {
	cases := []struct {
		in        string
		wantMajor uint32
		wantMinor uint32
		wantBuild uint32
		wantErr   bool
	}{
		{"v1.2.3", 1, 2, 3, false},
		{"1.2.3", 1, 2, 3, false},
		{"v2.0", 2, 0, 0, false},
		{"3", 3, 0, 0, false},
		{"v1.10.0", 1, 10, 0, false},
		{"", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
		{"v1.2.x", 0, 0, 0, true},
		{"1.2.3.4", 0, 0, 0, true},
	}
	for _, c := range cases {
		major, minor, build, err := parseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseVersion(%q) 应报错, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseVersion(%q) 报错: %v", c.in, err)
			continue
		}
		if major != c.wantMajor || minor != c.wantMinor || build != c.wantBuild {
			t.Errorf("parseVersion(%q) = %d.%d.%d, want %d.%d.%d",
				c.in, major, minor, build, c.wantMajor, c.wantMinor, c.wantBuild)
		}
	}
}
