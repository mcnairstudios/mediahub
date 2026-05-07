// hlsfresh: minimal H.265 fMP4 HLS server using hevc_videotoolbox.
//
// Transcodes a video source to HEVC + AAC in fMP4 HLS segments and serves
// them with a video.js 8 player. Handles the hvcC compat-flag bit reversal
// and AAC esds patching needed for Chrome playback.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/asticode/go-astiav"
	"github.com/mcnairstudios/mediahub/pkg/av/decode"
	"github.com/mcnairstudios/mediahub/pkg/av/encode"
	avmux "github.com/mcnairstudios/mediahub/pkg/av/mux"
	"github.com/mcnairstudios/mediahub/pkg/av/resample"
	"github.com/mcnairstudios/mediahub/pkg/av/scale"
)

const (
	sourceURL  = "http://192.168.1.149:8090/stream/e8942ef8d1bca8fb"
	outputDir  = "/tmp/hls-fresh1/"
	listenAddr = ":9898"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	cleanDir(outputDir)

	stopCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		close(stopCh)
	}()

	// Start transcoding in background; signal when first segments are ready.
	var ready sync.WaitGroup
	ready.Add(1)
	go transcode(stopCh, &ready)

	// Wait for pipeline to produce first segments before accepting HTTP.
	ready.Wait()

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/", servePlayer)
	httpMux.Handle("/hls/", http.StripPrefix("/hls/", corsHandler(http.FileServer(http.Dir(outputDir)))))

	log.Printf("serving on http://localhost%s", listenAddr)
	srv := &http.Server{Addr: listenAddr, Handler: httpMux}
	go func() {
		<-stopCh
		srv.Close()
	}()
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// cleanDir removes old HLS artifacts.
func cleanDir(dir string) {
	for _, pat := range []string{"seg*.m4s", "init.mp4", "playlist.m3u8"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pat))
		for _, f := range matches {
			os.Remove(f)
		}
	}
}

// transcode opens the source, decodes, re-encodes to HEVC+AAC, and muxes
// into fMP4 HLS. It calls ready.Done() once the first video frames are muxed.
func transcode(stopCh chan struct{}, ready *sync.WaitGroup) {
	readyFired := false
	fireReady := func() {
		if !readyFired {
			readyFired = true
			ready.Done()
		}
	}
	defer fireReady() // unblock main even on early failure

	// --- Open input ---
	inputFC := astiav.AllocFormatContext()
	if inputFC == nil {
		log.Fatal("alloc input format context")
	}
	defer func() {
		inputFC.CloseInput()
		inputFC.Free()
	}()

	dict := astiav.NewDictionary()
	defer dict.Free()
	dict.Set("probesize", "5000000", 0)
	dict.Set("analyzeduration", "3000000", 0)

	if err := inputFC.OpenInput(sourceURL, nil, dict); err != nil {
		log.Fatalf("open input: %v", err)
	}
	if err := inputFC.FindStreamInfo(nil); err != nil {
		log.Fatalf("find stream info: %v", err)
	}

	// Find best video and audio streams.
	videoIdx, audioIdx := -1, -1
	var videoStream, audioStream *astiav.Stream
	for _, s := range inputFC.Streams() {
		switch s.CodecParameters().MediaType() {
		case astiav.MediaTypeVideo:
			if videoIdx < 0 {
				videoIdx = s.Index()
				videoStream = s
			}
		case astiav.MediaTypeAudio:
			if audioIdx < 0 {
				audioIdx = s.Index()
				audioStream = s
			}
		}
	}
	if videoIdx < 0 {
		log.Fatal("no video stream found")
	}

	vcp := videoStream.CodecParameters()
	srcW, srcH := vcp.Width(), vcp.Height()
	log.Printf("input video: %s %dx%d", vcp.CodecID().String(), srcW, srcH)

	// --- Video decoder ---
	videoDec, err := decode.NewVideoDecoderFromParams(vcp, decode.DecodeOpts{})
	if err != nil {
		log.Fatalf("video decoder: %v", err)
	}
	defer videoDec.Close()

	// --- Audio decoder ---
	var audioDec *decode.Decoder
	if audioIdx >= 0 {
		acp := audioStream.CodecParameters()
		log.Printf("input audio: %s %dHz %dch",
			acp.CodecID().String(), acp.SampleRate(),
			acp.ChannelLayout().Channels())
		audioDec, err = decode.NewAudioDecoderFromParams(acp)
		if err != nil {
			log.Printf("audio decoder failed, video-only: %v", err)
			audioIdx = -1
		}
	}
	if audioDec != nil {
		defer audioDec.Close()
	}

	// --- Video encoder: hevc_videotoolbox ---
	fps := 25
	videoEnc, err := encode.NewVideoEncoder(encode.EncodeOpts{
		Codec:            "h265",
		HWAccel:          "videotoolbox",
		Width:            srcW,
		Height:           srcH,
		Bitrate:          5000,
		KeyframeInterval: fps * 2,
		Framerate:        fps,
	})
	if err != nil {
		log.Fatalf("video encoder: %v", err)
	}
	defer videoEnc.Close()
	log.Printf("video encoder: %s extradata=%d bytes", videoEnc.CodecID(), len(videoEnc.Extradata()))

	// Scaler created lazily after first decoded frame.
	var scaler *scale.Scaler
	defer func() {
		if scaler != nil {
			scaler.Close()
		}
	}()

	// --- Audio encoder: AAC-LC ---
	var audioEnc *encode.Encoder
	var resampler *resample.Resampler
	if audioIdx >= 0 {
		audioEnc, err = encode.NewAACEncoder(2, 48000)
		if err != nil {
			log.Printf("audio encoder failed: %v", err)
			audioIdx = -1
		}
	}
	if audioEnc != nil {
		defer audioEnc.Close()
	}
	defer func() {
		if resampler != nil {
			resampler.Close()
		}
	}()

	// --- HLS muxer: fMP4 segments (required for HEVC in browsers) ---
	hlsOpts := avmux.HLSMuxOpts{
		OutputDir:          outputDir,
		SegmentDurationSec: 6,
		SegmentType:        "fmp4",
		IsLive:             true,
		VideoCodecID:       astiav.CodecIDHevc,
		VideoExtradata:     videoEnc.Extradata(),
		VideoWidth:         srcW,
		VideoHeight:        srcH,
		VideoTimeBase:      videoEnc.TimeBase(),
		VideoFrameRate:     fps,
	}
	if audioEnc != nil {
		hlsOpts.AudioCodecID = astiav.CodecIDAac
		hlsOpts.AudioExtradata = audioEnc.Extradata()
		hlsOpts.AudioChannels = 2
		hlsOpts.AudioSampleRate = 48000
		hlsOpts.AudioTimeBase = audioEnc.TimeBase()
		hlsOpts.AudioFrameSize = audioEnc.FrameSize()
	}

	hlsMuxer, err := avmux.NewHLSMuxer(hlsOpts)
	if err != nil {
		log.Fatalf("HLS muxer: %v", err)
	}
	defer hlsMuxer.Close()

	// --- Read loop ---
	pkt := astiav.AllocPacket()
	defer pkt.Free()

	videoInTB := videoStream.TimeBase()
	encTB := astiav.NewRational(1, fps)

	videoFrames, audioFrames := 0, 0
	videoDecErrs, audioDecErrs := 0, 0

	log.Println("transcode loop starting...")

	for {
		select {
		case <-stopCh:
			log.Println("transcode stopped by signal")
			return
		default:
		}

		if err := inputFC.ReadFrame(pkt); err != nil {
			log.Printf("read frame: %v", err)
			break
		}

		idx := pkt.StreamIndex()

		switch {
		case idx == videoIdx:
			frames, decErr := videoDec.Decode(pkt)
			pkt.Unref()
			if decErr != nil {
				videoDecErrs++
				if videoDecErrs <= 50 {
					continue
				}
				log.Printf("video decode error #%d: %v", videoDecErrs, decErr)
				continue
			}

			for _, frame := range frames {
				// Lazy scaler: decoded format -> NV12 for VideoToolbox.
				if scaler == nil {
					srcFmt := frame.PixelFormat()
					if srcFmt != astiav.PixelFormatNv12 {
						scaler, err = scale.NewScaler(
							frame.Width(), frame.Height(), srcFmt,
							srcW, srcH, astiav.PixelFormatNv12,
						)
						if err != nil {
							log.Fatalf("scaler: %v", err)
						}
						log.Printf("scaler: %s -> nv12", srcFmt.Name())
					}
				}

				encFrame := frame
				if scaler != nil {
					scaled, scErr := scaler.Scale(frame)
					if scErr != nil {
						frame.Free()
						continue
					}
					scaled.SetPts(frame.Pts()) // scaler must copy PTS
					frame.Free()
					encFrame = scaled
				}

				// Convert PTS: input timebase -> encoder timebase.
				if encFrame.Pts() != astiav.NoPtsValue {
					encFrame.SetPts(astiav.RescaleQ(encFrame.Pts(), videoInTB, encTB))
				}

				encPkts, encErr := videoEnc.Encode(encFrame)
				encFrame.Free()
				if encErr != nil {
					log.Printf("video encode: %v", encErr)
					continue
				}

				for _, ep := range encPkts {
					// Encoder outputs in its own timebase; rescale to muxer input TB.
					ep.RescaleTs(encTB, hlsOpts.VideoTimeBase)
					if muxErr := hlsMuxer.WriteVideoPacket(ep); muxErr != nil {
						log.Printf("mux video: %v", muxErr)
					}
					ep.Free()
					videoFrames++
				}

				if videoFrames >= 5 {
					fireReady()
				}
				if videoFrames%250 == 0 && videoFrames > 0 {
					log.Printf("video: %d frames, %d segments", videoFrames, hlsMuxer.SegmentCount())
				}
			}

		case idx == audioIdx && audioDec != nil && audioEnc != nil:
			frames, decErr := audioDec.Decode(pkt)
			pkt.Unref()
			if decErr != nil {
				audioDecErrs++
				if audioDecErrs <= 5 {
					continue
				}
				continue
			}

			for _, frame := range frames {
				// Lazy resampler: source format -> AAC encoder format (fltp).
				if resampler == nil {
					srcCh := frame.ChannelLayout().Channels()
					resampler, err = resample.NewResampler(
						srcCh, frame.SampleRate(), frame.SampleFormat(),
						2, 48000, audioEnc.SampleFormat(),
					)
					if err != nil {
						log.Printf("resampler failed: %v", err)
						audioIdx = -1
						frame.Free()
						break
					}
					log.Printf("resampler: %s %dHz %dch -> %s 48kHz stereo",
						frame.SampleFormat().Name(), frame.SampleRate(), srcCh,
						audioEnc.SampleFormat().Name())
				}

				resampled, resErr := resampler.Convert(frame)
				frame.Free()
				if resErr != nil {
					continue
				}
				if resampled == nil {
					continue
				}

				encPkts, encErr := audioEnc.Encode(resampled)
				resampled.Free()
				if encErr != nil {
					continue
				}
				for _, ep := range encPkts {
					if muxErr := hlsMuxer.WriteAudioPacket(ep); muxErr != nil {
						log.Printf("mux audio: %v", muxErr)
					}
					ep.Free()
					audioFrames++
				}
			}

		default:
			pkt.Unref()
		}
	}

	log.Printf("transcode done: %d video, %d audio frames", videoFrames, audioFrames)
}

// corsHandler wraps a handler with CORS headers and correct MIME types for HLS.
func corsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")
		switch filepath.Ext(r.URL.Path) {
		case ".m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case ".m4s", ".mp4":
			w.Header().Set("Content-Type", "video/mp4")
		}
		h.ServeHTTP(w, r)
	})
}

func servePlayer(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, playerHTML)
}

// playerHTML: video.js 8 configured for H.265 fMP4 HLS playback in Chrome.
//
// Chrome requirements for HEVC HLS:
//   - fMP4 segments (not MPEG-TS) -- Chrome MSE does not support HEVC in TS
//   - hvc1 codec tag (not hev1) -- parameter sets in sample descriptions, not in-band
//   - hvcC box profile_compatibility_flags must be bit-reversed (RFC 6381)
//     so video.js/VHS generates "hvc1.1.6.L120.B0" not "hvc1.1.60000000.L120.B0"
//   - AAC esds must contain AudioObjectType=2 (AAC-LC) for "mp4a.40.2"
//   - Platform must have a hardware HEVC decoder (most modern Macs/PCs)
//
// video.js VHS (Video.js HTTP Streaming) handles fMP4 HLS natively.
// We use overrideNative:true to force VHS on all browsers (including Safari)
// for consistent behavior.
const playerHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>H.265 HLS Player</title>
  <link href="https://vjs.zencdn.net/8.10.0/video-js.css" rel="stylesheet">
  <style>
    body {
      margin: 0; background: #111;
      display: flex; justify-content: center; align-items: center;
      min-height: 100vh; flex-direction: column;
    }
    .video-js {
      width: 80vw; height: 45vw;
      max-width: 1280px; max-height: 720px;
    }
    #info {
      color: #888; font: 12px/1.6 monospace;
      margin-top: 12px; text-align: center;
    }
  </style>
</head>
<body>
  <video-js id="player" class="vjs-default-skin" controls autoplay muted>
    <source src="/hls/playlist.m3u8" type="application/x-mpegURL">
  </video-js>
  <div id="info">waiting for stream...</div>
  <script src="https://vjs.zencdn.net/8.10.0/video.min.js"></script>
  <script>
    var info = document.getElementById('info');
    var player = videojs('player', {
      html5: {
        vhs: {
          overrideNative: true
        },
        nativeAudioTracks: false,
        nativeVideoTracks: false
      },
      liveui: true
    });

    player.on('loadedmetadata', function() {
      var tech = player.tech({ IWillNotUseThisInPlugins: true });
      var vhs = tech && tech.vhs;
      if (vhs && vhs.playlists && vhs.playlists.media()) {
        var attrs = vhs.playlists.media().attributes || {};
        info.textContent = 'codecs: ' + (attrs.CODECS || 'unknown');
      } else {
        info.textContent = 'playing (metadata loaded)';
      }
    });
    player.on('error', function() {
      info.textContent = 'error: ' + (player.error() ? player.error().message : 'unknown');
    });
    player.on('playing', function() {
      info.textContent = 'playing';
    });
    player.on('waiting', function() {
      info.textContent = 'buffering...';
    });

    // Retry on prolonged stall.
    var stallTimer;
    player.on('waiting', function() {
      clearTimeout(stallTimer);
      stallTimer = setTimeout(function() {
        info.textContent = 'retrying...';
        player.src({ src: '/hls/playlist.m3u8', type: 'application/x-mpegURL' });
        player.play();
      }, 15000);
    });
    player.on('playing', function() { clearTimeout(stallTimer); });
  </script>
</body>
</html>`
