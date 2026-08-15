package handler

import (
	"testing"
)

// TestValidStreamURL 校验登记视频地址的协议合法性
func TestValidStreamURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"rtsp://192.168.1.100:554/ch1", true},
		{"rtsps://cam.example.com:554/live", true},
		{"https://cdn.example.com/hls/ch1.m3u8", true},
		{"http://cdn.example.com/flv/ch1.flv", true},
		{"ftp://cdn.example.com/video.mp4", false},
		{"file:///tmp/x.mp4", false},
		{"no-protocol://x", false},
		{"rtsp://", false},          // 空主机
		{"https://", false},         // 空主机
		{"rtsp://192.168.1.100:55", true}, // 主机存在
		{"", false},
	}
	for _, c := range cases {
		if got := validStreamURL(c.url); got != c.want {
			t.Errorf("validStreamURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
