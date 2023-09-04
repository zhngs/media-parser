// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// reflect demonstrates how with one PeerConnection you can send video to Pion and have the packets sent back
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ObserverData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type SignalData struct {
	Sdp  webrtc.SessionDescription `json:"sdp"`
	Uuid string                    `json:"uuid"`
}

func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func Decode(in string, obj interface{}) {
	err := json.Unmarshal([]byte(in), obj)
	if err != nil {
		panic(err)
	}
}

func HTTPSDPServer() (chan string, chan string) {
	port := flag.Int("port", 8443, "http server port")
	flag.Parse()

	sdpChan := make(chan string)
	http.HandleFunc("/signal", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer c.Close()

		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			log.Printf("signal message type: %d, recv: %s", mt, message)
			sdpChan <- string(message)

			// log.Printf("signal message type: %d, write: %s", mt, message)
			c.WriteMessage(mt, []byte(<-sdpChan))
		}
	})

	observerChan := make(chan string, 2)
	http.HandleFunc("/observer", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		log.Printf("handle observer")
		defer c.Close()
		for {
			message := <-observerChan
			log.Printf("write observer message %s", message)
			c.WriteMessage(websocket.TextMessage, []byte(message))
		}
	})

	go func() {
		// nolint: gosec
		err := http.ListenAndServe(":"+strconv.Itoa(*port), nil)
		if err != nil {
			panic(err)
		}
	}()

	return sdpChan, observerChan
}

type customLogger struct {
	logger *zap.Logger
}

func (c *customLogger) Trace(msg string) { c.logger.Debug(msg, zap.String("pionLevel", "trace")) }
func (c *customLogger) Tracef(format string, args ...interface{}) {
	c.logger.Debug(fmt.Sprintf(format, args...), zap.String("pionLevel", "trace"))
}
func (c *customLogger) Debug(msg string) { c.logger.Debug(msg) }
func (c *customLogger) Debugf(format string, args ...interface{}) {
	c.logger.Debug(fmt.Sprintf(format, args...))
}
func (c *customLogger) Info(msg string) { c.logger.Info(msg) }
func (c *customLogger) Infof(format string, args ...interface{}) {
	c.logger.Info(fmt.Sprintf(format, args...))
}
func (c *customLogger) Warn(msg string) { c.logger.Warn(msg) }
func (c *customLogger) Warnf(format string, args ...interface{}) {
	c.logger.Warn(fmt.Sprintf(format, args...))
}
func (c *customLogger) Error(msg string) { c.logger.Error(msg) }
func (c *customLogger) Errorf(format string, args ...interface{}) {
	c.logger.Error(fmt.Sprintf(format, args...))
}

type customLoggerFactory struct{}

func (c customLoggerFactory) NewLogger(subsystem string) logging.LeveledLogger {
	cfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Development:       true,
		DisableCaller:     false,
		DisableStacktrace: false,
		Encoding:          "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseColorLevelEncoder,
			EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.FullCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger := zap.Must(cfg.Build())
	logger = logger.WithOptions(zap.AddCallerSkip(1))
	// defer logger.Sync()
	return &customLogger{
		logger: logger,
	}
}

// nolint:gocognit
func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.
	sdpChan, observerChan := HTTPSDPServer()

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// We'll use a VP8 and Opus but you can also define your own
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	i := &interceptor.Registry{}

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	// Register a intervalpli factory
	// This interceptor sends a PLI every 3 seconds. A PLI causes a video keyframe to be generated by the sender.
	// This makes our video seekable and more error resilent, but at a cost of lower picture quality and higher bitrates
	// A real world application should process incoming RTCP packets from viewers and forward them to senders
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		panic(err)
	}
	i.Add(intervalPliFactory)

	statsInterceptorFactory, err := stats.NewInterceptor()
	if err != nil {
		panic(err)
	}
	var statsGetter stats.Getter
	statsInterceptorFactory.OnNewPeerConnection(func(_ string, g stats.Getter) {
		statsGetter = g
	})
	i.Add(statsInterceptorFactory)

	s := webrtc.SettingEngine{
		LoggerFactory: customLoggerFactory{},
	}
	s.SetEphemeralUDPPortRange(9000, 9000)
	// s.SetLite(true)
	// s.SetNAT1To1IPs([]string{"127.0.0.1"}, webrtc.ICECandidateTypeHost)

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(s), webrtc.WithInterceptorRegistry(i))

	// Prepare the configuration
	config := webrtc.Configuration{}
	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	// Create Track that we send video back to browser on
	outputTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	rtpSender, err := peerConnection.AddTrack(outputTrack)
	if err != nil {
		panic(err)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	offerData := SignalData{}
	Decode(<-sdpChan, &offerData)

	observerChan <- Encode(ObserverData{
		Type: "sdp",
		Data: offerData.Sdp,
	})

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offerData.Sdp)
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().MimeType)

		go func() {
			for {
				stats := statsGetter.Get(uint32(track.SSRC()))

				fmt.Printf("Stats for: %s\n", track.Codec().MimeType)
				fmt.Println(stats)

				time.Sleep(time.Second * 5)
			}
		}()

		for {
			// Read RTP packets being sent to Pion
			rtp, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			if writeErr := outputTrack.WriteRTP(rtp); writeErr != nil {
				panic(writeErr)
			}
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate

	<-gatherComplete

	answerData := SignalData{
		Sdp:  *peerConnection.LocalDescription(),
		Uuid: uuid.New().String(),
	}

	sdpChan <- Encode(answerData)
	observerChan <- Encode(ObserverData{
		Type: "sdp",
		Data: answerData.Sdp,
	})

	// Block forever
	select {}
}
