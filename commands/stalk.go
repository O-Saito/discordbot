package commands

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"mydiscordbot/bot"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"layeh.com/gopus"
)

const (
	sampleRate    = 48000
	numChannels   = 2
	bitsPerSample = 16
)

type WAVWriter struct {
	file      *os.File
	dataStart int64
	dataSize  int64
}

func NewWAVWriter(f *os.File) (*WAVWriter, error) {
	writer := &WAVWriter{
		file:      f,
		dataStart: 44,
		dataSize:  0,
	}

	if err := writer.writeHeader(); err != nil {
		return nil, err
	}

	return writer, nil
}

func (w *WAVWriter) writeHeader() error {
	header := make([]byte, 44)

	copy(header[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:8], 0xFFFFFFFF)
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], numChannels)
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], sampleRate*numChannels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(header[32:34], numChannels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(header[34:36], bitsPerSample)
	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], 0xFFFFFFFF)

	_, err := w.file.Write(header)
	return err
}

func (w *WAVWriter) WritePCM(pcm []int16) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, pcm); err != nil {
		return err
	}
	_, err := w.file.Write(buf.Bytes())
	if err != nil {
		return err
	}
	w.dataSize += int64(len(pcm) * 2)
	return nil
}

func (w *WAVWriter) Finalize() error {
	fileSize := w.dataSize + 36
	dataSize := w.dataSize

	if _, err := w.file.Seek(4, 0); err != nil {
		return err
	}
	if err := binary.Write(w.file, binary.LittleEndian, fileSize); err != nil {
		return err
	}

	if _, err := w.file.Seek(40, 0); err != nil {
		return err
	}
	if err := binary.Write(w.file, binary.LittleEndian, dataSize); err != nil {
		return err
	}

	return nil
}

func (w *WAVWriter) Close() error {
	return w.file.Close()
}

type StalkCommand struct {
	bot.CommandBase
}

func (c *StalkCommand) Name() string        { return "stalk" }
func (c *StalkCommand) Description() string { return "Record voice chat" }

func (c *StalkCommand) GetApplicationCommand() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        c.Name(),
		Description: "Toggle voice recording (starts or stops recording)",
	}
}

func (c *StalkCommand) ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any {
	result := map[string]any{}

	if member := e.Member(); member != nil {
		result["member"] = member.User.ID
	}

	return &result
}

func (c *StalkCommand) Execute(cs *bot.CommandState) error {
	g := cs.G

	if g.IsRecording {
		return handleStalkStop(cs)
	}

	return handleStalkStart(cs)
}

func handleStalkStart(cs *bot.CommandState) error {
	g := cs.G
	botUserID := cs.Client.ID()

	if g.VoiceConn == nil {
		if err := JoinVoiceChannel(cs); err != nil {
			cs.SingleResponse("Failed to join voice: " + err.Error())
			return nil
		}
	}

	baseName := fmt.Sprintf("recording_%s_%d",
		time.Now().Format("20060102_150405"),
		os.Getpid())

	os.MkdirAll("audios", 0755)

	g.RecordingUserFiles = make(map[snowflake.ID]*bot.UserRecording)
	g.RecordingBaseName = baseName
	g.IsRecording = true
	g.RecordingDone = make(chan struct{})

	go stalkRecordingLoop(g, botUserID)

	cs.SingleResponse("Started recording (one file per user)")
	return nil
}

func stalkRecordingLoop(g *bot.GuildState, botUserID snowflake.ID) {
	defer close(g.RecordingDone)

	log.Printf("[Stalk] Recording loop started")
	for g.IsRecording {
		packet, err := g.VoiceConn.UDP().ReadPacket()
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		userID := g.VoiceConn.UserIDBySSRC(packet.SSRC)
		if userID == 0 { //|| userID == botUserID {
			continue
		}

		if g.RecordingUserFiles == nil {
			log.Printf("[Stalk] RecordingUserFiles is nil, exiting loop")
			return
		}

		rec, exists := g.RecordingUserFiles[userID]
		if !exists {
			filename := fmt.Sprintf("%s_%s.wav", g.RecordingBaseName, userID.String())
			fp := filepath.Join("audios", filename)

			f, err := os.Create(fp)
			if err != nil {
				log.Printf("[Stalk] Failed to create file for user %s: %v", userID, err)
				continue
			}

			decoder, err := gopus.NewDecoder(sampleRate, numChannels)
			if err != nil {
				log.Printf("[Stalk] Failed to create decoder for user %s: %v", userID, err)
				f.Close()
				continue
			}

			wavWriter, err := NewWAVWriter(f)
			if err != nil {
				log.Printf("[Stalk] Failed to create WAV writer for user %s: %v", userID, err)
				f.Close()
				continue
			}

			rec = &bot.UserRecording{
				File:         f,
				Decoder:      decoder,
				DataSize:     0,
				WavDataStart: wavWriter.dataStart,
			}
			g.RecordingUserFiles[userID] = rec
			log.Printf("[Stalk] Created new file for user %s: %s", userID, filename)
		}

		pcm, err := rec.Decoder.Decode(packet.Opus, 960, false)
		if err != nil {
			log.Printf("[Stalk] Failed to decode opus for user %s: %v", userID, err)
			continue
		}

		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, pcm)
		rec.File.Write(buf.Bytes())
		rec.DataSize += int64(len(pcm) * 2)
	}
	log.Printf("[Stalk] Recording loop ended")
}

func handleStalkStop(cs *bot.CommandState) error {
	g := cs.G

	if !g.IsRecording {
		cs.SingleResponse("Not currently recording")
		return nil
	}

	g.IsRecording = false

	if g.RecordingDone != nil {
		log.Printf("[Stalk] Waiting for recording loop to finish...")
		<-g.RecordingDone
		log.Printf("[Stalk] Recording loop finished")
	}

	var convertedFiles []string
	for userID, rec := range g.RecordingUserFiles {
		if rec == nil || rec.File == nil {
			continue
		}

		wavPath := rec.File.Name()

		if err := rec.File.Sync(); err != nil {
			log.Printf("[Stalk] Failed to sync file for user %s: %v", userID, err)
		}
		rec.File.Close()

		oggPath := filepath.Join("audios", fmt.Sprintf("%s_%s.ogg", g.RecordingBaseName, userID.String()))

		log.Printf("[Stalk] Converting WAV to OGG: %s -> %s", wavPath, oggPath)

		cmd := exec.Command("ffmpeg",
			"-y",
			"-i", wavPath,
			"-c:a", "libopus",
			"-b:a", "128k",
			oggPath)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("[Stalk] FFmpeg error for user %s: %v", userID, err)
			log.Printf("[Stalk] FFmpeg stderr: %s", stderr.String())
			convertedFiles = append(convertedFiles, fmt.Sprintf("%s.wav (conversion failed)", userID.String()))
			continue
		}

		log.Printf("[Stalk] Conversion successful for user %s", userID)
		os.Remove(wavPath)
		convertedFiles = append(convertedFiles, filepath.Base(oggPath))
	}

	g.RecordingUserFiles = nil
	g.RecordingBaseName = ""

	if len(convertedFiles) == 0 {
		cs.SingleResponse("Stopped recording (no audio captured)")
	} else {
		cs.SingleResponse("Stopped recording: " + joinStrings(convertedFiles, ", "))
	}

	return nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func (c *StalkCommand) HandleButton(cs *bot.CommandState, customID string) error { return nil }
func (c *StalkCommand) HandleSelectMenu(cs *bot.CommandState, customID string, values []string) error {
	return nil
}
func (c *StalkCommand) HandleModalSubmit(cs *bot.CommandState, customID string, data map[string]string) error {
	return nil
}
