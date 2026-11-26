package main

import (
	"math"
	"testing"
)

const probeFixture = `{
  "streams": [
    {
      "index": 0, "codec_name": "hevc", "codec_type": "video", "profile": "Main 10",
      "width": 3840, "height": 2160, "pix_fmt": "yuv420p10le",
      "color_transfer": "smpte2084", "avg_frame_rate": "24000/1001"
    },
    {
      "index": 1, "codec_name": "dts", "codec_type": "audio", "profile": "DTS-HD MA",
      "channels": 2, "channel_layout": "stereo", "sample_rate": "48000",
      "bits_per_raw_sample": "24",
      "tags": {"language": "ita", "title": "意大利语 DTS-HD 2.0"},
      "disposition": {"default": 1}
    },
    {
      "index": 2, "codec_name": "ac3", "codec_type": "audio",
      "channels": 2, "channel_layout": "stereo", "sample_rate": "48000",
      "bits_per_sample": "16",
      "tags": {"language": "eng", "title": "英语 DD 2.0"},
      "disposition": {"default": 1}
    },
    {
      "index": 3, "codec_name": "hdmv_pgs_subtitle", "codec_type": "subtitle",
      "tags": {"language": "chi", "title": "简英双语"},
      "disposition": {"default": 1, "forced": 0}
    }
  ],
  "format": {
    "format_name": "matroska,webm", "duration": "5609.088",
    "size": "58864563961", "bit_rate": "83955985"
  }
}`

func TestParseProbeJSON(t *testing.T) {
	media, err := parseProbeJSON([]byte(probeFixture))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(media.DurationSeconds-5609.088) > 0.001 {
		t.Errorf("duration = %f", media.DurationSeconds)
	}
	if media.SizeBytes != 58864563961 || media.BitRate != 83955985 {
		t.Errorf("size/bitrate = %d/%d", media.SizeBytes, media.BitRate)
	}
	if media.Video == nil {
		t.Fatal("no video")
	}
	v := media.Video
	if v.Codec != "HEVC" || v.Profile != "Main 10" || v.Width != 3840 || v.Height != 2160 ||
		v.BitDepth != 10 || v.HDR != "HDR10" || math.Abs(v.FPS-23.976) > 0.01 {
		t.Errorf("video = %+v", v)
	}
	if len(media.Audio) != 2 {
		t.Fatalf("audio count = %d", len(media.Audio))
	}
	if media.Audio[0].Language != "it" || media.Audio[0].Codec != "DTS-HD MA" ||
		media.Audio[0].Channels != "2.0" || media.Audio[0].SampleRate != 48000 ||
		media.Audio[0].BitDepth != 24 || !media.Audio[0].Default {
		t.Errorf("audio[0] = %+v", media.Audio[0])
	}
	if media.Audio[1].Language != "en" || media.Audio[1].Codec != "DD" ||
		media.Audio[1].BitDepth != 16 {
		t.Errorf("audio[1] = %+v", media.Audio[1])
	}
	if len(media.Subtitles) != 1 {
		t.Fatalf("subtitle count = %d", len(media.Subtitles))
	}
	if media.Subtitles[0].Language != "zh" || media.Subtitles[0].Format != "PGS" ||
		!media.Subtitles[0].Default {
		t.Errorf("subtitle[0] = %+v", media.Subtitles[0])
	}
}

func TestProbeHelpers(t *testing.T) {
	if got := normalizeLang("ita"); got != "it" {
		t.Errorf("normalizeLang(ita) = %q", got)
	}
	if got := normalizeLang("chi"); got != "zh" {
		t.Errorf("normalizeLang(chi) = %q", got)
	}
	if got := normalizeLang("eng"); got != "en" {
		t.Errorf("normalizeLang(eng) = %q", got)
	}
	if got := channelsLabel("stereo", 2); got != "2.0" {
		t.Errorf("channelsLabel = %q", got)
	}
	if got := parseFrameRate("24000/1001"); math.Abs(got-23.976) > 0.01 {
		t.Errorf("parseFrameRate = %f", got)
	}
	exact, total := formatDurationHuman(5609.088)
	if exact != "1:33:29" || total != "93 min" {
		t.Errorf("formatDurationHuman = %q, %q", exact, total)
	}
}
