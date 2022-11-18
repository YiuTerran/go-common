package sdp

import "fmt"

/**  gb28181扩展的字段
  *  @author tryao
  *  @date 2022/03/24 15:12
**/

const (
	StreamTypeLive   = 0
	StreamTypeRecord = 1

	VF_MPEG4 = 1
	VF_H264  = 2
	VF_SVAC  = 3
	VF_3GP   = 4

	RS_QCIF  = 1
	RS_CIF   = 2
	RS_CIF4  = 3
	RS_D1    = 4
	RS_720P  = 5
	RS_1080P = 6

	BitRateCBR = 1
	BitRateVBR = 2

	AF_G711  = 1
	AF_G7231 = 2
	AF_G729  = 3
	AF_G7221 = 4
)

var (
	AudioBitRateMap = map[int]float32{
		1: 5.3,
		2: 6.3,
		3: 8,
		4: 16,
		5: 24,
		6: 32,
		7: 48,
		8: 64,
	}
	AudioSampleRateMap = map[int]int{
		1: 8,
		2: 14,
		3: 16,
		4: 32,
	}
)

func NewSSRC(streamType int, domain string, index int) string {
	return fmt.Sprintf("%d%s%04d", streamType, domain, index)
}

type MediaFormat struct {
	VideoEncoder     int //视频编码格式
	ResolutionType   int //分辨率
	VideoFrameRate   int //帧率
	VideoBitRateType int //码率类型
	VideoBitRate     int //码率
	AudioEncoder     int //音频编码格式
	AudioBitRate     int //音频编码码率
	AudioSampleRate  int //音频采样率
}

func (mf *MediaFormat) String() string {
	return fmt.Sprintf("v/%d/%d/%d/%d/%da/%d/%d/%d", mf.VideoEncoder, mf.ResolutionType, mf.VideoFrameRate,
		mf.VideoBitRateType, mf.VideoBitRate, mf.AudioEncoder, mf.AudioBitRate, mf.AudioSampleRate)
}
