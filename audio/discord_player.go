package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo/voice"
	"layeh.com/gopus"
)

const (
	sampleRate = 48000
	channels   = 2
	frameSize  = 960
	maxBytes   = frameSize * 2 * 2
)

type DiscordPlayer struct {
	volume      float64
	playing     int32
	stopped     int32
	ffmpegPath  string
	opusEncoder *gopus.Encoder
	onFinished  func()
	ffmpegCmd   *exec.Cmd
	pcmSend     chan []int16
	pcmClose    chan bool
	isHTTP      bool
	volumeMu    sync.Mutex
	playerMu    sync.Mutex
	ffmpegMu    sync.Mutex
	finished    chan struct{}
	voiceConn   voice.Conn
}

func NewDiscordPlayer(ffmpegPath string) (*DiscordPlayer, error) {
	encoder, err := gopus.NewEncoder(sampleRate, channels, gopus.Audio)
	if err != nil {
		return nil, err
	}

	return &DiscordPlayer{
		volume:      1.0,
		ffmpegPath:  ffmpegPath,
		opusEncoder: encoder,
		finished:    make(chan struct{}),
	}, nil
}

func (p *DiscordPlayer) Play(reader io.Reader) error {
	return nil
}

func (p *DiscordPlayer) PlayURL(url string, sampleRate int) error {
	return p.PlayURLWithSeek(url, sampleRate, 0)
}

func (p *DiscordPlayer) PlayURLWithSeek(url string, sampleRate int, seekSeconds int) error {
	p.Stop()

	atomic.StoreInt32(&p.stopped, 0)

	p.volumeMu.Lock()
	vol := p.volume
	p.volumeMu.Unlock()

	isHTTP := strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
	p.playerMu.Lock()
	p.isHTTP = isHTTP
	p.playerMu.Unlock()

	ffmpegReader, ffmpegWriter := io.Pipe()
	stderrBuf := &bytes.Buffer{}

	p.ffmpegMu.Lock()
	cmdArgs := []string{}
	if seekSeconds > 0 {
		cmdArgs = append(cmdArgs, "-ss", strconv.Itoa(seekSeconds))
	}

	if isHTTP {
		cmdArgs = append(cmdArgs,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-fflags", "+genpts",
			"-loglevel", "error",
			"-i", url,
			"-vn",
			"-filter:a", "volume="+strconv.FormatFloat(vol, 'f', -1, 64),
			"-f", "s16le",
			"-ac", "2",
			"pipe:1")
	} else {
		cmdArgs = append(cmdArgs,
			"-loglevel", "error",
			"-i", url,
			"-vn",
			"-filter:a", "volume="+strconv.FormatFloat(vol, 'f', -1, 64),
			"-f", "s16le",
			"-ac", "2",
			"pipe:1")
	}

	p.ffmpegCmd = exec.Command(p.ffmpegPath, cmdArgs...)
	p.ffmpegCmd.Stdout = ffmpegWriter
	p.ffmpegCmd.Stderr = stderrBuf

	if err := p.ffmpegCmd.Start(); err != nil {
		p.ffmpegMu.Unlock()
		return err
	}
	p.ffmpegMu.Unlock()

	p.pcmSend = make(chan []int16, 2)
	p.pcmClose = make(chan bool)

	go p.sendPCM(context.Background(), nil)

	atomic.StoreInt32(&p.playing, 1)

	go func() {
		defer ffmpegReader.Close()
		defer close(p.pcmSend)

		frameCount := 0
		ffmpegBuf := io.Reader(ffmpegReader)

		for {
			audioBuf := make([]int16, frameSize*channels)
			err := binary.Read(ffmpegBuf, binary.LittleEndian, audioBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				continue
			}

			frameCount++
			select {
			case p.pcmSend <- audioBuf:
			case <-p.pcmClose:
				return
			}
		}

		if atomic.LoadInt32(&p.stopped) == 0 {
			atomic.StoreInt32(&p.playing, 0)
			p.Stop()
		}
	}()

	return nil
}

func (p *DiscordPlayer) SetVoiceConn(conn voice.Conn) {
	p.playerMu.Lock()
	defer p.playerMu.Unlock()
	p.voiceConn = conn
}

func (p *DiscordPlayer) PlayURLWithSeekAndVC(ctx context.Context, url string, sampleRate int, seekSeconds int, vc voice.Conn) error {
	fmt.Printf("[Audio] PlayURLWithSeekAndVC called with url: %s\n", url)
	p.Stop()

	atomic.StoreInt32(&p.stopped, 0)

	p.volumeMu.Lock()
	vol := p.volume
	p.volumeMu.Unlock()

	isHTTP := strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
	p.playerMu.Lock()
	p.isHTTP = isHTTP
	p.playerMu.Unlock()

	ffmpegReader, ffmpegWriter := io.Pipe()
	stderrBuf := &bytes.Buffer{}

	p.ffmpegMu.Lock()
	cmdArgs := []string{}
	if seekSeconds > 0 {
		cmdArgs = append(cmdArgs, "-ss", strconv.Itoa(seekSeconds))
	}

	if isHTTP {
		cmdArgs = append(cmdArgs,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-fflags", "+genpts",
			"-loglevel", "error",
			"-i", url,
			"-vn",
			"-filter:a", "volume="+strconv.FormatFloat(vol, 'f', -1, 64),
			"-f", "s16le",
			"-ac", "2",
			"pipe:1")
	} else {
		cmdArgs = append(cmdArgs,
			"-loglevel", "error",
			"-i", url,
			"-vn",
			"-filter:a", "volume="+strconv.FormatFloat(vol, 'f', -1, 64),
			"-f", "s16le",
			"-ac", "2",
			"pipe:1")
	}

	p.ffmpegCmd = exec.Command(p.ffmpegPath, cmdArgs...)
	p.ffmpegCmd.Stdout = ffmpegWriter
	p.ffmpegCmd.Stderr = stderrBuf

	if err := p.ffmpegCmd.Start(); err != nil {
		p.ffmpegMu.Unlock()
		return err
	}
	p.ffmpegMu.Unlock()

	if vc == nil {
		return fmt.Errorf("voice connection is nil")
	}

	if err := vc.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
		return err
	}

	p.pcmSend = make(chan []int16, 2)
	p.pcmClose = make(chan bool)

	go p.sendPCM(ctx, vc)

	atomic.StoreInt32(&p.playing, 1)

	go func(ffmpegReader *io.PipeReader) {
		p.ffmpegCmd.Wait()
		fmt.Printf("[Audio] FFMPEG WAITED\n")

		ffmpegReader.Close()

		p.ffmpegMu.Lock()
		if p.pcmClose != nil {
			close(p.pcmClose)
			p.pcmClose = nil
		}
		p.ffmpegMu.Unlock()

	}(ffmpegReader)

	go func() {
		defer vc.SetSpeaking(ctx, voice.SpeakingFlagMicrophone|voice.SpeakingFlagSoundshare)
		defer close(p.pcmSend)

		frameCount := 0
		ffmpegBuf := io.Reader(ffmpegReader)
		for {
			audioBuf := make([]int16, frameSize*channels)
			err := binary.Read(ffmpegBuf, binary.LittleEndian, audioBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				fmt.Printf("[Audio] io.EOF err\n")
				break
			}
			if err != nil {
				fmt.Printf("[Audio] read error: %v, breaking loop\n", err)
				break
			}

			frameCount++
			select {
			case p.pcmSend <- audioBuf:
			case <-p.pcmClose:
				fmt.Printf("[Audio] pcmClose received, breaking loop\n")
				return
			}
		}

		fmt.Printf("[Audio] ffmpeg loop ended, frameCount: %d\n", frameCount)

		if atomic.LoadInt32(&p.stopped) == 0 {
			atomic.StoreInt32(&p.playing, 0)
			select {
			case p.finished <- struct{}{}:
			default:
			}
		}
	}()

	return nil
}

func (p *DiscordPlayer) sendPCM(ctx context.Context, vc voice.Conn) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.pcmClose:
			return
		case <-ticker.C:
			recv, ok := <-p.pcmSend
			if !ok {
				return
			}

			if !p.IsPlaying() {
				continue
			}

			opus, err := p.opusEncoder.Encode(recv, frameSize, maxBytes)
			if err != nil {
				continue
			}

			if vc == nil {
				return
			}

			_, err = vc.UDP().Write(opus)
			if err != nil {
				return
			}
		}
	}
}

func (p *DiscordPlayer) Pause() {
	atomic.StoreInt32(&p.playing, 0)
}

func (p *DiscordPlayer) Resume() {
	atomic.StoreInt32(&p.playing, 1)
}

func (p *DiscordPlayer) Stop() {
	callback := p.onFinished
	p.onFinished = nil

	if !atomic.CompareAndSwapInt32(&p.stopped, 0, 1) {
		return
	}

	p.ffmpegMu.Lock()
	if p.pcmClose != nil {
		close(p.pcmClose)
		p.pcmClose = nil
	}
	if p.ffmpegCmd != nil && p.ffmpegCmd.Process != nil {
		p.ffmpegCmd.Process.Kill()
		p.ffmpegCmd = nil
	}
	p.ffmpegMu.Unlock()

	atomic.StoreInt32(&p.playing, 0)
	if callback != nil {
		callback()
	}
}

func (p *DiscordPlayer) SetVolume(volume float64) {
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}

	p.volumeMu.Lock()
	p.volume = volume
	p.volumeMu.Unlock()
}

func (p *DiscordPlayer) Volume() float64 {
	p.volumeMu.Lock()
	defer p.volumeMu.Unlock()
	return p.volume
}

func (p *DiscordPlayer) IsPlaying() bool {
	return atomic.LoadInt32(&p.playing) == 1
}

func (p *DiscordPlayer) SetOnFinishedCallback(fn func()) {
	p.onFinished = fn
}

func (p *DiscordPlayer) Finished() <-chan struct{} {
	return p.finished
}
