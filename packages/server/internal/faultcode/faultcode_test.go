package faultcode

import "testing"

func TestFaultTypeFromErrCode(t *testing.T) {
	cases := []struct {
		code int8
		want string
	}{
		{LEDErrOK, "unknown"},           // 0 正常
		{LEDErrROFF, "lamp_off"},        // -1
		{LEDErrYOFF, "lamp_off"},        // -2
		{LEDErrGOFF, "lamp_off"},        // -3
		{LEDErrRYON, "abnormal_on"},     // -4
		{LEDErrRGON, "abnormal_on"},     // -5
		{LEDErrYGON, "abnormal_on"},     // -6
		{LEDErrRYGON, "abnormal_on"},    // -7
		{LEDErrRONTimeout, "timeout"},   // -8
		{LEDErrYONTimeout, "timeout"},   // -9
		{LEDErrGONTimeout, "timeout"},   // -10
		{LEDErrRDim, "dim"},             // -11
		{LEDErrYDim, "dim"},             // -12
		{LEDErrGDim, "dim"},             // -13
		{LEDErrPowerLoss, "power_loss"}, // -14
		{100, "unknown"},
		{-100, "unknown"},
	}
	for _, c := range cases {
		if got := FaultTypeFromErrCode(c.code); got != c.want {
			t.Errorf("FaultTypeFromErrCode(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestFaultLevelFromErrCode(t *testing.T) {
	cases := []struct {
		code int8
		want string
	}{
		{LEDErrROFF, "critical"},
		{LEDErrYOFF, "critical"},
		{LEDErrGOFF, "critical"},
		{LEDErrRYON, "critical"},
		{LEDErrRGON, "critical"},
		{LEDErrYGON, "critical"},
		{LEDErrRYGON, "critical"},
		{LEDErrPowerLoss, "critical"},
		{LEDErrRONTimeout, "normal"},
		{LEDErrYONTimeout, "normal"},
		{LEDErrGONTimeout, "normal"},
		{LEDErrRDim, "normal"},
		{LEDErrYDim, "normal"},
		{LEDErrGDim, "normal"},
		{LEDErrOK, "normal"},
		{7, "normal"},
	}
	for _, c := range cases {
		if got := FaultLevelFromErrCode(c.code); got != c.want {
			t.Errorf("FaultLevelFromErrCode(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestConstants_Semantics(t *testing.T) {
	// 断言关键语义值不被意外改动
	if LEDErrOK != 0 || LEDErrROFF != -1 || LEDErrPowerLoss != -14 {
		t.Fatal("错误码常量语义被修改")
	}
	if StateR != 0 || StateY != 1 || StateG != 2 || StateNone != -1 {
		t.Fatal("灯组状态常量语义被修改")
	}
}
